package encryption

import (
	"errors"
	"fmt"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/golang-jwt/jwt/v4"
)

// StandardClaims returns a standard JWT claim for a username.
//
// It will include values for Subject (username), Issuer (site title), ExpiresAt, IssuedAt, NotBefore.
//
// If the userID is >0, the ID field is included.
func StandardClaims(userID uint64, username string, expiresAt time.Time) jwt.RegisteredClaims {
	claim := jwt.RegisteredClaims{
		Subject:   username,
		Issuer:    config.Title,
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now()),
	}
	if userID > 0 {
		claim.ID = fmt.Sprintf("%d", userID)
	}
	return claim
}

// SignClaims creates and returns a signed JWT token.
func SignClaims(claims jwt.Claims, secret []byte) (string, error) {
	// Get our Chat JWT secret.
	if len(secret) == 0 {
		return "", errors.New("JWT secret key is not configured")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}

	return ss, nil
}

// ValidateClaims checks a JWT token is signed by the site key and returns the claims.
func ValidateClaims(tokenStr string, secret []byte, v jwt.Claims) (jwt.Claims, bool, error) {
	// Handle a JWT authentication token.
	var (
		claims jwt.Claims
		authOK bool
	)
	if tokenStr != "" {
		token, err := jwt.ParseWithClaims(tokenStr, v, func(token *jwt.Token) (interface{}, error) {
			return secret, nil
		})
		if err != nil {
			return nil, false, err
		}

		if !token.Valid {
			return nil, false, errors.New("token was not valid")
		}

		claims = token.Claims
		authOK = true
	}

	return claims, authOK, nil
}
