package forum

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// MoveThread controller (/forum/admin/move-thread) to move a thread to another forum.
func MoveThread() http.HandlerFunc {
	// Reuse the upload page but with an EditPhoto variable.
	tmpl := templates.Must("forum/move_thread.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			idStr    = r.FormValue("thread_id")
			fragment = r.PostFormValue("fragment")
			threadID uint64
		)

		// Parse the thread ID.
		if a, err := strconv.Atoi(idStr); err != nil {
			session.FlashError(w, r, "Invalid thread ID.")
			templates.Redirect(w, "/")
			return
		} else {
			threadID = uint64(a)
		}

		// Load this thread.
		thread, err := models.GetThread(threadID)
		if err != nil {
			session.FlashError(w, r, "Couldn't find that forum thread: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Confirming the move?
		if r.Method == http.MethodPost {
			forum, err := models.ForumByFragment(fragment)
			if err != nil {
				session.FlashError(w, r, "No forum was found with the fragment: %s", fragment)
				templates.Redirect(w, fmt.Sprintf("%s?thread_id=%d", r.URL.Path, threadID))
				return
			}

			// Move the thread.
			if err := thread.Move(forum); err != nil {
				session.FlashError(w, r, "Error moving the thread: %s", err)
			} else {
				session.Flash(w, r, "The thread has been moved to forum: %s (/f/%s)", forum.Title, forum.Fragment)
			}

			templates.Redirect(w, fmt.Sprintf("/forum/thread/%d", threadID))
			return
		}

		var vars = map[string]interface{}{
			"Thread": thread,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
