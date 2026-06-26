package index

import (
	"net/http"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models/demographic"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Create the controller.
func Create() http.HandlerFunc {
	tmpl := templates.Must("index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" || r.Method != http.MethodGet {
			// Try and correct for a trailing /
			if strings.HasSuffix(r.URL.Path, "/") {
				next := strings.TrimRight(r.URL.Path, "/")
				if next == "" {
					next = "/"
				}
				templates.Redirect(w, next)
				return
			}

			log.Error("404 Not Found: %s", r.URL.Path)
			templates.NotFoundPage(w, r)
			return
		}

		// Get website statistics to show on home page.
		demo, err := demographic.Get()
		if err != nil {
			log.Error("demographic.Get: %s", err)
		}

		vars := map[string]interface{}{
			"Demographic": demo,
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// Favicon
func Favicon() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, config.StaticPath+"/favicon.ico")
	})
}

// Robots
func Robots() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, config.StaticPath+"/robots.txt")
	})
}

// PWA Manifest
func Manifest() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, config.StaticPath+"/manifest.json")
	})
}

// Service Worker for web push.
func ServiceWorker() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/javascript; charset=UTF-8")
		w.Header().Add("Service-Worker-Allowed", "/")
		http.ServeFile(w, r, config.StaticPath+"/js/service-worker.js")
	})
}
