package index

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/templates"
)

// StaticTemplate creates a simple controller that loads a Go html/template
// such as "about.html" relative to the web/templates path.
func StaticTemplate(filename string) func() http.HandlerFunc {
	return func() http.HandlerFunc {
		tmpl := templates.Must(filename)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := tmpl.Execute(w, r, nil); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		})
	}
}
