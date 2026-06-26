package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
)

// Recovery recovery middleware.
func Recovery(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Error("PANIC: %v", err)
				debug.PrintStack()

				// Attempt to file an admin report for visibility.
				stack := debug.Stack()
				go ReportRecovery(r, err, stack)

				http.Error(w, "Internal Server Error (golang)", http.StatusInternalServerError)
			}
		}()
		handler.ServeHTTP(w, r)
	})
}

// ReportRecovery tries to generate an admin feedback report to capture a recovered panic.
func ReportRecovery(r *http.Request, err interface{}, stack []byte) {
	// Log an admin report for this user.
	fb := &models.Feedback{
		Intent:  "report",
		Subject: "Internal Server Error",
		Message: fmt.Sprintf(
			"An uncaught panic has been recovered (an Internal Server Error) on route: **%s**\n\n"+
				"> %v\n\n"+
				"Stack trace:\n\n"+
				"```\n%s\n```",
			r.URL.Path,
			err,
			stack,
		),
	}
	if err := models.CreateFeedback(fb); err != nil {
		log.Error("ReportRecovery: error saving admin report: %s", err)
	}
}
