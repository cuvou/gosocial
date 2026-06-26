package templates

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/session"
)

// Template is a logical HTML template for the app with ability to wrap around an html/template
// and provide middlewares, hooks or live reloading capability in debug mode.
type Template struct {
	filename string       // Filename on disk (index.html)
	filepath string       // Full path on disk (./web/templates/index.html)
	modified time.Time    // Modification date of the file at init time
	mu       sync.RWMutex // Lock templates during reloading
	tmpl     *template.Template
}

// LoadTemplate processes and returns a template. Filename is relative
// to the template directory, e.g. "index.html". Call this at the initialization
// of your endpoint controller; in debug mode the template HTML from disk may be
// reloaded if modified after initial load.
func LoadTemplate(filename string) (*Template, error) {
	filepath := config.TemplatePath + "/" + filename
	stat, err := os.Stat(filepath)
	if err != nil {
		return nil, fmt.Errorf("LoadTemplate(%s): %s", filename, err)
	}

	files := templates(config.TemplatePath + "/" + filename)
	tmpl := template.New("page")
	tmpl.Funcs(TemplateFuncs(nil))
	_, err = tmpl.ParseFiles(files...)
	if err != nil {
		log.Error("LoadTemplate(%s): ParseFiles: %s", filename, err)
	}

	return &Template{
		filename: filename,
		filepath: filepath,
		modified: stat.ModTime(),
		tmpl:     tmpl,
	}, nil
}

// LoadCustom loads a bare template without the site theme and partial templates attached.
//
// The custom TempleFuncs and vars are still available (PrettyTitle, .CurrentUser, etc.)
func LoadCustom(filename string) (*Template, error) {
	filepath := config.TemplatePath + "/" + filename
	stat, err := os.Stat(filepath)
	if err != nil {
		return nil, fmt.Errorf("LoadTemplate(%s): %s", filename, err)
	}

	// Load the template plus common partials.
	files := templatesCommon(config.TemplatePath + "/" + filename)

	tmpl := template.New("page")
	tmpl.Funcs(TemplateFuncs(nil))
	tmpl.ParseFiles(files...)

	return &Template{
		filename: filename,
		filepath: filepath,
		modified: stat.ModTime(),
		tmpl:     tmpl,
	}, nil
}

// Must LoadTemplate or panic.
func Must(filename string) *Template {
	tmpl, err := LoadTemplate(filename)
	if err != nil {
		panic(err)
	}
	return tmpl
}

// Must LoadCustom or panic.
func MustLoadCustom(filename string) *Template {
	tmpl, err := LoadCustom(filename)
	if err != nil {
		panic(err)
	}
	return tmpl
}

// Execute a loaded template. In debug mode, the template file may be reloaded
// from disk if the file on disk has been modified.
func (t *Template) Execute(w io.Writer, r *http.Request, vars map[string]interface{}) error {

	if vars == nil {
		vars = map[string]interface{}{}
	}

	// Merge in global variables.
	MergeVars(r, vars)
	MergeUserVars(r, vars)

	// Merge the flashed messsage variables in.
	if r != nil {
		if rw, ok := w.(http.ResponseWriter); ok {
			sess := session.Get(r)
			flashes, errors := sess.ReadFlashes(rw, r)
			vars["Flashes"] = flashes
			vars["Errors"] = errors
		}
	}

	// Reload the template from disk?
	if config.Debug && t.IsModifiedLocally() {
		if err := t.Reload(); err != nil {
			log.Error("Reloading error: %s", err)
		}
	}

	// Lock the base template for reading in case of a concurrent Reload, and then
	// clone the template for the per-request Funcs to avoid race conditions.
	t.mu.RLock()
	baseTemplate := t.tmpl
	t.mu.RUnlock()

	tmpl := baseTemplate

	// Install the function map.
	if r != nil {
		tmpl = template.Must(baseTemplate.Clone())
		tmpl = tmpl.Funcs(TemplateFuncs(r))
	}

	if err := tmpl.ExecuteTemplate(w, "base", vars); err != nil {
		return err
	}

	return nil
}

// IsModifiedLocally checks if any of the template partials of your Template have
// had their files locally on disk modified, so to know to reload them.
func (t *Template) IsModifiedLocally() bool {
	// Check all the template files from base.html, to partials, to our filepath.
	var files = templates(t.filepath)

	latest, err := latestModTime(files)
	if err != nil {
		log.Error("Template(%s): stat error: %v", t.filename, err)
		return false
	}

	if latest.After(t.modified) {
		log.Info("Template(%s).Execute: files updated on disk, reloading", t.filename)
		return true
	}

	return false
}

// Reload the template from disk.
func (t *Template) Reload() error {

	// Lock templates during reloading.
	t.mu.Lock()
	defer t.mu.Unlock()

	var files = templates(t.filepath)

	latest, err := latestModTime(files)
	if err != nil {
		return fmt.Errorf("Reload(%s): %v", t.filename, err)
	}

	tmpl := template.New("page")
	tmpl.Funcs(TemplateFuncs(nil))
	if _, err := tmpl.ParseFiles(files...); err != nil {
		return err
	}

	t.tmpl = tmpl
	t.modified = latest
	return nil
}

// Latest modification time of any template in the list of partials.
func latestModTime(files []string) (time.Time, error) {
	var latest time.Time
	for _, f := range files {
		stat, err := os.Stat(f)
		if err != nil {
			return time.Time{}, err
		}
		if stat.ModTime().After(latest) {
			latest = stat.ModTime()
		}
	}
	return latest, nil
}

// Base template layout.
var commonPartials = []string{
	// Partial templates useful for both main and HTMX components.
	config.TemplatePath + "/partials/user_avatar.html",
	config.TemplatePath + "/partials/markdown_editor.html",
}
var baseTemplates = append([]string{
	config.TemplatePath + "/base.html",
	config.TemplatePath + "/partials/alert_modal.html",
	config.TemplatePath + "/partials/like_modal.html",
	config.TemplatePath + "/partials/right_click.html",
	config.TemplatePath + "/partials/themes.html",
	config.TemplatePath + "/partials/forum_tabs.html",
	config.TemplatePath + "/partials/settings_menu.html",
	config.TemplatePath + "/partials/profile_tabs.html",
	config.TemplatePath + "/partials/poll_creator.html",
}, commonPartials...)

// templates returns a template chain with the base templates preceding yours.
// Files given are expected to be full paths (config.TemplatePath + file)
func templates(files ...string) []string {
	return append(baseTemplates, files...)
}

// customTemplates loads your files along with only the common partials. This is
// used especially for custom/HTMX pages which will define their own "base" template
// but may still want access to common partials.
func templatesCommon(files ...string) []string {
	return append(commonPartials, files...)
}

// RenderTemplate executes a template. Filename is relative to the templates
// root, e.g. "index.html"
func RenderTemplate(w io.Writer, r *http.Request, filename string, vars map[string]interface{}) error {
	if vars == nil {
		vars = map[string]interface{}{}
	}

	// Merge in user vars.
	MergeVars(r, vars)
	MergeUserVars(r, vars)

	files := templates(config.TemplatePath + "/" + filename)
	tmpl := template.Must(
		template.New("index").ParseFiles(files...),
	)

	err := tmpl.ExecuteTemplate(w, "base", vars)
	if err != nil {
		return err
	}

	return nil
}
