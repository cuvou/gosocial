// Package markdown provides markdown render functions.
package markdown

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/microcosm-cc/bluemonday"
	"github.com/shurcooL/github_flavored_markdown"
)

var (
	RegexpHTMLTag      = regexp.MustCompile(`<(.|\n)+?>`)
	RegexpMarkdownLink = regexp.MustCompile(`\[([^\[\]]*)\]\((.*?)\)`)
	RegexpBBCodeColor  = regexp.MustCompile(`\[/?color(?:=[^\]]+?)?\]`)
)

// Render markdown and BBCode from untrusted sources.
func Render(input string) string {

	// BBCode first: it outputs HTML that Markdown can still work with.
	input = RenderBBCode(input)
	input = UnescapeBBCode(input) // Allow raw HTML thru for later

	// The only thing BBCode does that won't survive Markdown is colors.
	input, colors := CollectColorTags(input)

	// Run it through the Markdown compiler.
	input = RenderMarkdownUnsafe(input)

	// Sanitize the HTML with bluemonday.
	input = SanitizeHTML(input)

	// Substitute back in the BBCode [color] tags.
	input = ReplaceColorTags(input, colors)

	// Special easter egg: substitute in the [gosocial] BBCode tag.
	input = strings.ReplaceAll(input, config.BBCodePrettyTitle, config.PrettyTitle)

	return input
}

// RenderMarkdownUnsafe simply runs the Markdown processor but does not run bluemonday to sanitize it.
//
// You probably want the Render() function.
func RenderMarkdownUnsafe(input string) string {
	var html = github_flavored_markdown.Markdown([]byte(input))
	return string(html)
}

// SanitizeHTML uses bluemonday to sanitize the final HTML result, removing any dangerous code like script tags and stylesheets.
func SanitizeHTML(html string) string {
	p := bluemonday.UGCPolicy()
	safened := p.Sanitize(html)
	return safened
}

// Quotify a message putting it into a Markdown "> quotes" block.
func Quotify(input string) string {
	var lines = []string{}
	for _, line := range strings.Split(input, "\n") {
		lines = append(lines, "> "+line)
	}
	return strings.Join(lines, "\n")
}

/*
DeMarkify strips some Markdown syntax from plain text snippets.

It is especially useful on Forum views that show plain text contents of posts, and especially
for linked at-mentions of the form `[@username](/go/comment?id=1234)` so that those can be
simplified down to just the `@username` text contents.

This function will strip and simplify:

- Markdown hyperlinks as spelled out above
- Literal HTML tags such as <a href="">
*/
func DeMarkify(input string) string {
	// Strip all standard HTML tags.
	input = StripHTML(input)

	// Replace spammy Markdown hyperlinks and BBCode color tags.
	input = RegexpMarkdownLink.ReplaceAllString(input, "$1")
	input = RegexpBBCodeColor.ReplaceAllLiteralString(input, "")

	return input
}

// StripHTML removes all HTML from a string, leaving only the plain text parts.
func StripHTML(html string) string {
	return RegexpHTMLTag.ReplaceAllString(html, "")
}

// MapToBulletedList converts a hash map into a Markdown formatted bulleted list of key/value pairs.
func MapToBulletedList(m map[string]string) string {
	var lines = []string{}
	for k, v := range m {
		lines = append(lines, fmt.Sprintf("* %s: %v", k, v))
	}
	return strings.Join(lines, "\n")
}
