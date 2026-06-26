package spam

import (
	"errors"
	"regexp"
	"strings"

	"github.com/cuvou/gosocial/pkg/models"
)

// VideoChatWebsites to third-party video hosting apps: we already have our own chat room, and third-party links shared in
// public places can pose a risk to user privacy/safety.
var VideoChatWebsites = []string{
	"join.skype.com",
	"zoom.us",
	"whereby.com",
	"meet.jit.si",
	"https://t.me",
}

// PaidContentWebsites to spammy sites that solicit money for content.
var PaidContentWebsites = []string{
	"onlyfans.com",
	"justfor.fans",
	"justforfans",
}

// ContactSpam is a list of common messenger apps that people will sometimes mention on their Photo Gallery captions
// or similar when trying to skirt around the rules and advertise their contact info too publicly.
var (
	ContactSpam = []*regexp.Regexp{
		regexp.MustCompile(`(?:tele\s*gram|tele\s*guard)`),
		regexp.MustCompile(`(?:whats\s*app)`),
	}

	PaidContentSpam = []*regexp.Regexp{
		regexp.MustCompile(`(?:only\s*fans|of$|of[!.]$)`), // Onlyfans and ending with 'OF' or 'OF!'
		regexp.MustCompile(`(?:just\s*for\s*fans|\bjff\b)`),
	}
)

/*
DetectSpamLinks searches a message (such as a comment, forum post, etc.) for spammy contents such as Skype invite links
and returns an error if found.

This checks for website links specifically (like join.skype.com, meet.jit.si, onlyfans.com) and is intended to return
a hard error that prevents the message containing those links from being published on the website at all.
*/
func DetectSpamLinks(message string) error {
	message = strings.ToLower(message)
	for _, link := range VideoChatWebsites {
		if strings.Contains(message, link) {
			return errors.New(
				"Your message could not be posted because it contains a link to a third-party video chat website. " +
					"In the interest of protecting our community, we do not allow linking to third-party video conferencing apps where user " +
					"privacy and security may not hold up to our standards, or where the content may run against our terms of service.",
			)
		}
	}

	for _, link := range PaidContentWebsites {
		if strings.Contains(message, link) {
			return errors.New(
				"Your message could not be posted because it contains a link to a paid content website such as Onlyfans. " +
					"Websites of this class are widely regarded to be spam and must not be posted anywhere on the site except for one's own " +
					"profile page.",
			)
		}
	}

	return nil
}

/*
DetectContactSpam searches a string for mentions of spammy contact links (such as Telegram).

It is intended to prevent users from e.g. mentioning their Telegram username on Photo Gallery comments.
It also detects mentions of common spammy websites such as Onlyfans.

The caller should flash the error message to the user, and then blank out (don't store) the user's value,
so e.g. if they are doing this to a Photo Caption then you just store a blank string for the caption.

This function also checks for 'hard links' to sites via DetectSpamLinks.
*/
func DetectContactSpam(message string) error {
	message = strings.ToLower(message)
	for _, link := range ContactSpam {
		if link.MatchString(message) {
			return errors.New(
				"It is against the site's rules to advertise your contact information here (such as your Telegram or WhatsApp ID). " +
					"Fields where you have mentioned these platforms have not been saved and will be left empty.",
			)
		}
	}

	for _, link := range PaidContentSpam {
		if link.MatchString(message) {
			return errors.New(
				"It is against the rules to advertise your paid content website (such as Onlyfans) here. " +
					"Fields where you have mentioned these platforms have not been saved and will be left empty.",
			)
		}
	}

	return DetectSpamLinks(message)
}

// RestrictSearchTerm can remove/replace search words to exclude blacklisted terms.
func RestrictSearchTerms(terms *models.Search) (*models.Search, error) {
	var (
		m      = map[string]interface{}{}
		result = &models.Search{}
		r1, r2 int
	)

	// Map the blacklist for easy lookup.
	for _, term := range restrictedSearchTerms {
		m[term] = nil
	}

	// Filter the includes+excludes.
	result.Includes, r1 = restrictTermsFrom(terms.Includes, m)
	result.Excludes, r2 = restrictTermsFrom(terms.Excludes, m)

	// If we have excluded everything down to zero.
	if len(terms.Includes) > 0 && len(result.Includes) == 0 {
		result.Includes = append(result.Includes, "36c49b88-dd0c-4b9f-a4e1-f0c73c976ce2")
	}

	// Were there restrictions?
	if r1+r2 > 0 {
		return result, errors.New("some search terms were restricted")
	}

	return result, nil
}

// Restricted search terms.
var restrictedSearchTerms = []string{}

func restrictTermsFrom(terms []string, blacklist map[string]interface{}) ([]string, int) {
	var (
		result []string
		count  int
	)

	for _, term := range terms {
		// Break words in "quoted strings" too.
		words := strings.Split(term, " ")
		for _, word := range words {
			if _, ok := blacklist[strings.ToLower(word)]; ok {
				count++
				continue
			}
			result = append(result, word)
		}
	}

	return result, count
}
