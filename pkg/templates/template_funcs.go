package templates

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/markdown"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/photo"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/utility"
)

// Generics
type Number interface {
	int | int64 | uint64 | float32 | float64
}

// TemplateFuncs available to all pages.
func TemplateFuncs(r *http.Request) template.FuncMap {
	return template.FuncMap{
		"InputCSRF":          InputCSRF(r),
		"SincePrettyCoarse":  SincePrettyCoarse(),
		"FormatNumberShort":  FormatNumberShort(),
		"FormatNumberCommas": FormatNumberCommas(),
		"FormatFilesize":     utility.FormatFilesize,
		"ComputeAge":         utility.Age,
		"Split":              strings.Split,
		"NewHashMap":         NewHashMap,
		"ToMarkdown":         ToMarkdown,
		"DeMarkify":          markdown.DeMarkify,
		"ToJSON":             ToJSON,
		"ToHTML":             ToHTML,
		"ToHTMLAttr":         ToHTMLAttr,
		"ToHTMLSafeURL":      ToHTMLSafeURL,
		"ToUnsafeJavaScript": ToUnsafeJavaScript,
		"ToString": func(v any) string {
			return fmt.Sprintf("%v", v)
		},
		"Sprintf":          fmt.Sprintf,
		"PhotoURL":         PhotoURL(r),
		"VisibleAvatarURL": photo.VisibleAvatarURL,
		"Now":              time.Now,
		"RunTime":          RunTime,
		"PrettyTitle": func() template.HTML {
			return template.HTML(config.PrettyTitle)
		},
		"PrettyTitleBranding": func() template.HTML {
			return template.HTML(config.PrettyTitleBranding())
		},
		"Pluralize":     Pluralize[int],
		"Pluralize64":   Pluralize[int64],
		"PluralizeU64":  Pluralize[uint64],
		"Substring":     Substring,
		"TrimEllipses":  TrimEllipses,
		"IterRange":     IterRange,
		"SubtractInt":   SubtractInt,
		"SubtractInt64": SubtractInt64,
		"UrlEncode":     UrlEncode,
		"QueryPlus":     QueryPlus(r),
		"SimplePager":   SimplePager(r),
		"HasSuffix":     strings.HasSuffix,

		"HaversineDistanceString": utility.HaversineDistanceString,

		// Test if a photo should be blurred ({{BlurExplicit .Photo}})
		"BlurExplicit": BlurExplicit(r),

		// Get a description for an admin scope (e.g. for transparency page).
		"AdminScopeDescription": config.AdminScopeDescription,

		// Embedded Gallery permalinks from Markdown messages.
		"LazyLoadGalleryLinksFooter":       LazyLoadGalleryLinksFooter,
		"ToMarkdownWithGalleryLinksInline": ToMarkdownWithGalleryLinksInline,

		// Gallery lightbox helper functions for frontend.
		"PhotosToJSONForLightbox": photo.PhotosToJSONForLightbox,

		// "ReSignPhotoLinks": photo.ReSignPhotoLinks,
		"ReSignPhotoLinks": func(s template.HTML) template.HTML {
			if currentUser, err := session.CurrentUser(r); err == nil {
				return template.HTML(photo.ReSignPhotoLinks(currentUser, string(s)))
			}
			return s
		},

		// "Color of the Day" theme selection.
		"GetWebsiteThemeForUser": GetWebsiteThemeForUser,

		// Extract hyperlinks from a block of text and return them as a list.
		"ExtractHyperlinksFromText": ExtractHyperlinksFromText,

		// Safe Mode enabled?
		"IsSafeMode": func() bool {
			ses := session.Get(r)
			return ses.SafeMode
		},

		// Feature flags.
		"IsFeatureFlagExplicitEnabled": func() bool {
			return config.FeatureFlagExplicitEnabled
		},
		"IsFeatureFlagBareRTCEnabled": func() bool {
			return config.Current.BareRTC.URL != "" && config.Current.BareRTC.JWTSecret != ""
		},
	}
}

// InputCSRF returns the HTML snippet for a CSRF token hidden input field.
func InputCSRF(r *http.Request) func() template.HTML {
	return func() template.HTML {
		ctx := r.Context()
		if token, ok := ctx.Value(session.CSRFKey).(string); ok {
			return template.HTML(fmt.Sprintf(
				`<input type="hidden" name="%s" value="%s">`,
				config.CSRFInputName,
				token,
			))
		} else {
			return template.HTML(`[CSRF middleware error]`)
		}
	}
}

// RunTime returns the elapsed time between the HTTP request start and now, as a formatted string.
func RunTime(r *http.Request) string {
	if rt, ok := r.Context().Value(session.RequestTimeKey).(time.Time); ok {
		duration := time.Since(rt)
		return duration.Round(time.Millisecond).String()
	}
	return "ERROR"
}

// PhotoURL returns a URL path to photos.
func PhotoURL(r *http.Request) func(filename string) string {
	return func(filename string) string {
		// Get the current user to sign a JWT token.
		var token string
		if currentUser, err := session.CurrentUser(r); err == nil {
			return photo.SignedPhotoURL(currentUser, filename)
		}

		return photo.URLPath(filename) + token
	}
}

// BlurExplicit returns true if the current user has the blur_explicit setting on and the given Photo or Video is Explicit.
func BlurExplicit(r *http.Request) func(any) bool {
	return func(photo any) bool {

		// This func works on both Photo and Video models. Collect its Explicit status.
		var isExplicit bool
		switch v := photo.(type) {
		case *models.Photo:
			isExplicit = v.Explicit
		case models.Photo:
			// e.g. User Profile Pictures which are dereferenced Photos.
			isExplicit = v.Explicit
		default:
			return false
		}

		// If safe mode, all photos are explicit.
		ses := session.Get(r)
		if ses.SafeMode {
			return true
		}

		if !isExplicit {
			return false
		}

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			return false
		}

		return currentUser.GetProfileField("blur_explicit") == "true"
	}
}

// SincePrettyCoarse formats a time.Duration in plain English. Intended for "joined 2 months ago" type
// strings - returns the coarsest level of granularity.
func SincePrettyCoarse() func(time.Time) template.HTML {
	return func(since time.Time) template.HTML {
		return template.HTML(utility.FormatDurationCoarse(time.Since(since)))
	}
}

// FormatNumberShort will format any integer number into a short representation.
func FormatNumberShort() func(v interface{}) template.HTML {
	return func(v interface{}) template.HTML {
		var number int64
		switch t := v.(type) {
		case int:
			number = int64(t)
		case int64:
			number = int64(t)
		case uint:
			number = int64(t)
		case uint64:
			number = int64(t)
		case float32:
			number = int64(t)
		case float64:
			number = int64(t)
		default:
			return template.HTML("#INVALID#")
		}
		return template.HTML(utility.FormatNumberShort(number))
	}
}

// FormatNumberCommas will pretty print a long number by adding commas.
func FormatNumberCommas() func(v interface{}) template.HTML {
	return func(v interface{}) template.HTML {
		return template.HTML(utility.FormatNumberCommas(v))
	}
}

// ToMarkdown renders input text as Markdown.
func ToMarkdown(input string) template.HTML {
	return template.HTML(markdown.Render(input))
}

// ToHTML renders input text as trusted HTML code.
func ToHTML(input string) template.HTML {
	return template.HTML(input)
}

// ToHTMLAttr renders input text as trusted HTML attribute code.
func ToHTMLAttr(input string) template.HTMLAttr {
	return template.HTMLAttr(input)
}

// ToHTMLSafeURL renders input text as trusted HTML URLibute code.
func ToHTMLSafeURL(input string) template.URL {
	return template.URL(input)
}

// ToJSON will stringify any json-serializable object.
func ToJSON(v any) template.JS {
	bin, err := json.Marshal(v)
	if err != nil {
		return template.JS(err.Error())
	}
	return template.JS(string(bin))
}

// ToUnsafeJavaScript allows a string to be used in JavaScript on frontend templates.
func ToUnsafeJavaScript(input string) template.JS {
	return template.JS(input)
}

// ExtractHyperlinksFromText parses a text blob for hyperlinks and returns them as a list.
func ExtractHyperlinksFromText(text string) []string {
	re := regexp.MustCompile(`https?://[^\s"'>]+`)
	return re.FindAllString(text, -1)
}

// Pluralize text based on a quantity number. Provide up to 2 labels for the
// singular and plural cases, or the defaults are "", "s"
func Pluralize[V Number](count V, labels ...string) string {
	if len(labels) < 2 {
		labels = []string{"", "s"}
	}

	if count == 1 {
		return labels[0]
	}
	return labels[1]
}

// Substring safely returns the first N characters of a string.
func Substring(value string, n int) string {
	if n > len(value) {
		return value
	}
	return value[:n]
}

// TrimEllipses is like Substring but will add an ellipses if truncated.
func TrimEllipses(value string, n int) string {
	if n > len(value) {
		return value
	}
	return value[:n] + "…"
}

// IterRange returns a list of integers useful for pagination.
func IterRange(start, n int) []int {
	var result = []int{}
	for i := start; i <= n; i++ {
		result = append(result, i)
	}
	return result
}

// SubtractInt subtracts two numbers.
func SubtractInt(a, b int) int {
	return a - b
}

// SubtractInt64 subtracts two numbers.
func SubtractInt64(a, b int64) int64 {
	return a - b
}

// NewHashMap creates a key/value dict on the fly for Go templates.
//
// Use it like: {{$Vars := NewHashMap "username" .CurrentUser.Username "photoID" .Photo.ID}}
//
// It is useful for calling Go subtemplates that need custom parameters, e.g. a
// mixin from current scope with other variables.
func NewHashMap(upsert ...interface{}) map[string]interface{} {
	// Map the positional arguments into a dictionary.
	var params = map[string]interface{}{}
	for i := 0; i < len(upsert); i += 2 {
		var (
			key   = fmt.Sprintf("%v", upsert[i])
			value interface{}
		)
		if len(upsert) > i {
			value = upsert[i+1]
		}

		params[key] = value
	}

	return params
}

// UrlEncode escapes a series of values (joined with no delimiter)
func UrlEncode(values ...interface{}) string {
	var result string
	for _, value := range values {
		result += url.QueryEscape(fmt.Sprintf("%v", value))
	}
	return result
}

// QueryPlus takes the current request's query parameters and upserts them with new values.
//
// Use it like: {{QueryPlus "page" .NextPage}}
//
// Returns the query string sans the ? prefix, like "key1=value1&key2=value2"
func QueryPlus(r *http.Request) func(...interface{}) template.URL {
	return func(upsert ...interface{}) template.URL {
		// Get current parameters.
		// Note: COPY them from r.Form so we don't accidentally modify r.Form.
		var params = map[string][]string{}
		for k, v := range r.Form {
			params[k] = v
		}

		// Mix in the incoming fields.
		for i := 0; i < len(upsert); i += 2 {
			var (
				key   = fmt.Sprintf("%v", upsert[i])
				value interface{}
			)
			if len(upsert) > i {
				value = upsert[i+1]
			}

			params[key] = []string{fmt.Sprintf("%v", value)}
		}

		// Assemble and return the query string.
		var parts = []string{}
		for k, vs := range params {
			for _, v := range vs {
				if v == "" {
					continue
				}
				parts = append(parts,
					fmt.Sprintf("%s=%s", url.QueryEscape(k), url.QueryEscape(v)),
				)
			}
		}

		// Sort them deterministically.
		sort.Strings(parts)

		return template.URL(strings.Join(parts, "&"))
	}
}

// SimplePager creates a paginator row (partial template).
//
// Use it like: {{SimplePager .Pager}}
//
// It runs the template partial 'simple_pager.html' to customize it for the site theme.
func SimplePager(r *http.Request) func(*models.Pagination) template.HTML {
	return func(pager *models.Pagination) template.HTML {
		tmpl, err := template.New("index").Funcs(template.FuncMap{
			"QueryPlus": QueryPlus(r),
		}).ParseFiles(config.TemplatePath + "/partials/simple_pager.html")
		if err != nil {
			return template.HTML(err.Error())
		}

		var (
			vars = struct {
				Pager   *models.Pagination
				Request *http.Request
			}{pager, r}

			buf = bytes.NewBuffer([]byte{})
		)

		err = tmpl.ExecuteTemplate(buf, "SimplePager", vars)
		if err != nil {
			return template.HTML(err.Error())
		}
		return template.HTML(buf.String())
	}
}

// GetWebsiteThemeForUser returns the name of the user's website color theme, if set.
//
// The color theme is one of enum.WebsiteThemeHueChoices which is stored on the user's
// account as the ProfileField "website-theme-hue".
//
// Some of the color themes are dynamic, such as "rotate-colors" which selects one of
// the color schemes per day. It uses the current DayOfYear modulo the number of accent
// color schemes, and returns the name of one of the color themes.
//
// If the currentUser is nil (logged out), returns the DefaultWebsiteThemeHue theme.
func GetWebsiteThemeForUser(r *http.Request) string {
	currentUser, err := session.CurrentUser(r)
	if err != nil {
		return config.DefaultWebsiteThemeHue
	}

	theme := currentUser.GetProfileFieldOr("website-theme-hue", "__unset__")
	switch theme {
	case "__unset__":
		// User didn't set a theme, return the site's default.
		return config.DefaultWebsiteThemeHue
	case "none":
		// User expressly selected the default Bulma theme (no extra CSS style added)
		return ""
	case "rotate-colors":
		// rotate-colors theme: select a different one each day.
		day := time.Now().YearDay()
		return config.WebsiteThemeRotateColors[day%len(config.WebsiteThemeRotateColors)]
	}

	return theme
}
