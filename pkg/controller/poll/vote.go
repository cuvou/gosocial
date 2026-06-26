package poll

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Vote controller for polls.
func Vote() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			// Form parameters
			pollID       uint64
			fromThreadID uint64
			fromBlogID   uint64
			answers      = r.Form["answer"] // a slice in case of MultipleChoice
			nextURL      string
		)

		// Parse integer params.
		if value, err := strconv.Atoi(r.FormValue("poll_id")); err != nil {
			session.FlashError(w, r, "Invalid poll ID")
			templates.Redirect(w, "/")
			return
		} else {
			pollID = uint64(value)
		}

		// Polls can exist on forum threads or blog posts. Parse the relevant ID.
		if value, err := strconv.Atoi(r.FormValue("from_thread_id")); err == nil {
			fromThreadID = uint64(value)
			nextURL = fmt.Sprintf("/forum/thread/%d", fromThreadID)
		} else if value, err := strconv.Atoi(r.FormValue("from_blog_id")); err == nil {
			fromBlogID = uint64(value)
			nextURL = fmt.Sprintf("/go/blog?id=%d", fromBlogID)
		}

		// An owner for the poll is required.
		if fromThreadID == 0 && fromBlogID == 0 {
			session.FlashError(w, r, "Unknown location for this poll.")
			templates.Redirect(w, "/")
			return
		}

		// POST request only.
		if r.Method != http.MethodPost {
			session.FlashError(w, r, "POST requests only.")
			templates.Redirect(w, nextURL)
			return
		}

		// An answer is required.
		if len(answers) == 0 || len(answers) == 1 && answers[0] == "" {
			session.FlashError(w, r, "An answer to this poll is required for voting.")
			templates.Redirect(w, nextURL)
			return
		}

		// Get the current user.
		user, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: couldn't get CurrentUser")
			templates.Redirect(w, nextURL)
			return
		}

		// Look up the poll.
		poll, err := models.GetPoll(pollID)
		if err != nil {
			session.FlashError(w, r, "Poll not found.")
			templates.Redirect(w, nextURL)
			return
		}

		// Is it accepting responses?
		result := poll.Result(user)
		if !result.AcceptingVotes {
			session.FlashError(w, r, "This poll is not accepting your vote at this time.")
			templates.Redirect(w, nextURL)
			return
		}

		// Cast the vote!
		if err := poll.CastVote(user, answers); err != nil {
			session.FlashError(w, r, "Couldn't cast the vote: %s", err)
			templates.Redirect(w, nextURL)
			return
		}

		session.Flash(w, r, "Your vote has been recorded!")
		templates.Redirect(w, nextURL)
	})
}
