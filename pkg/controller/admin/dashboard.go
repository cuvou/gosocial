package admin

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/templates"
)

// Admin dashboard or landing page (/admin).
func Dashboard() http.HandlerFunc {
	tmpl := templates.Must("admin/dashboard.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var vars = map[string]any{}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
