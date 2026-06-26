package markdown

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/frustra/bbcode"
	"github.com/google/uuid"
)

var BBCodeCompiler bbcode.Compiler

// Regexps useful for BBCode.
var (
	RegexpHTMLColor  = regexp.MustCompile(`<span style="color: [A-Za-z0-9#.,\(\)%]+?;">`)
	RegexpHTMLCenter = regexp.MustCompile(`<div style="text-align: center;">`)
)

func init() {
	var (
		// BBCode Compiler options
		autoCloseTags              = true
		ignoreUnmatchedClosingTags = true
	)

	// Initialize the BBCode compiler.
	BBCodeCompiler = bbcode.NewCompiler(autoCloseTags, ignoreUnmatchedClosingTags)
}

// RenderBBCode processes BBCode tags.
//
// Sequences of `<br><br>` are converted back into \n so the output may also be treated with Markdown.
func RenderBBCode(input string) string {
	var result = BBCodeCompiler.Compile(input)
	return result
}

// UnescapeBBCode reverses a few HTML escapings that RenderBBCode does.
func UnescapeBBCode(html string) string {

	// If the user wrote a literal `<br>` in their text, preserve their intent at the end.
	html = strings.ReplaceAll(html, "&lt;br&gt;", "[[br]]")

	// BBCode will escape all HTML tags like <table>, undo the tag escapes.
	html = strings.ReplaceAll(html, "&lt;", "<")
	html = strings.ReplaceAll(html, "&gt;", ">")

	// Undo the quote character, e.g. if the tag is `<img src="x">`
	html = strings.ReplaceAll(html, "&#34;", `"`)

	// BBCode turns every line break of the input into a `<br>` tag, turn these back into
	// newlines as that is what Markdown will require to work.
	html = strings.ReplaceAll(html, "<br>", "\n")

	// Restore the `<br>` tags the user literally intended to be sent through Markdown.
	html = strings.ReplaceAll(html, "[[br]]", "<br>")

	return html
}

// CollectColorTags searches for BBCode rendered color tags.
//
// Returns a map of 'placeholder values' which look like '[BBCodeColor:$uuid:0]' with a random UUID and incrementing counter.
//
// The map values are the original `<span style="color: red">` tag which the placeholder is standing in for.
func CollectColorTags(html string) (string, map[string]string) {
	var (
		result  = map[string]string{}
		uuid    = uuid.New().String()
		matches [][]string
	)

	// Find and replace [color] codes.
	matches = RegexpHTMLColor.FindAllStringSubmatch(html, -1)
	for i, m := range matches {
		var (
			span        = m[0]
			placeholder = fmt.Sprintf("[BBCodeColor:%s:%d]", uuid, i)
		)

		result[placeholder] = span
		html = strings.Replace(html, span, placeholder, 1)
	}

	// Also collect [center] tags.
	matches = RegexpHTMLCenter.FindAllStringSubmatch(html, -1)
	for i, m := range matches {
		var (
			span        = m[0]
			placeholder = fmt.Sprintf("[BBCodeCenterTag:%s:%d]", uuid, i)
		)

		result[placeholder] = span
		html = strings.Replace(html, span, placeholder, 1)
	}

	return html, result
}

// ReplaceColorTags swaps the collected color tags back into the HTML string.
func ReplaceColorTags(html string, placeholders map[string]string) string {
	for placeholder, span := range placeholders {
		html = strings.Replace(html, placeholder, span, 1)
	}
	return html
}
