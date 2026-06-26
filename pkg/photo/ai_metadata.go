package photo

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/png"
	"io"
	"regexp"

	"github.com/cuvou/gosocial/pkg/markdown"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"
)

/*
TestAndReportAIPhoto checks an uploaded file for known A.I. tags and will file an
admin report if tags are discovered.

It is a common handler for Certification Photos, secondary Photo IDs, and gallery.
*/
func TestAndReportAIPhoto(currentUser *models.User, context string, filename string, file io.ReadSeeker, tableName string, tableID uint64) error {
	found, tags, err := ExtractAIMetadata(file)
	if found {
		// Generate an admin report.
		fb := &models.Feedback{
			Intent:    "report",
			Subject:   fmt.Sprintf("A.I. Generated %s", context),
			UserID:    currentUser.ID,
			TableName: tableName,
			TableID:   tableID,
			Message: fmt.Sprintf("This user has tried to pass off an A.I. generated image as their %s!\n\n"+
				"Metadata embedded in their file includes known A.I. image gen attributes. Please review the following details for accuracy:\n\n"+
				"* Filename: %s\n%s",
				context,
				filename,
				markdown.MapToBulletedList(tags),
			),
		}
		if err := models.CreateFeedback(fb); err != nil {
			return fmt.Errorf("AI photo detected from %s, but error saving admin report: %s", currentUser.Username, err)
		}
	}
	return err
}

type ExifWalker struct {
	fn func(exif.FieldName, *tiff.Tag) error
}

func (ew ExifWalker) Walk(name exif.FieldName, tag *tiff.Tag) error {
	return ew.fn(name, tag)
}

func ExifWalkerFunc(fn func(exif.FieldName, *tiff.Tag) error) ExifWalker {
	return ExifWalker{fn}
}

/*
ExtractAIMetadata checks an image file for metadata tags added by some A.I. tools.

It will check in the following places:

1. EXIF tags (JPEG/PNG/TIFF common metadata)
2. PNG text chunks (Stable Diffusion stores prompts/settings here)
3. XMP XML blocks (some generators embed structured data there)

Returns:

- A boolean: true means A.I. tags were identified.
- A map of the keys/values of embedded tags.
- Errors in case of errors.
*/
func ExtractAIMetadata(file io.ReadSeeker) (bool, map[string]string, error) {
	var (
		found   bool
		results = map[string]string{}
	)

	// 1. Try EXIF
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		return false, nil, err
	}
	if x, err := exif.Decode(file); err == nil {
		x.Walk(ExifWalkerFunc(func(name exif.FieldName, tag *tiff.Tag) error {
			val := tag.String()
			results[string(name)] = val
			if containsAIMetadata(val) {
				found = true
			}
			return nil
		}))

		if found {
			return found, results, nil
		}
	}

	// 2. PNG text chunks.
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return false, nil, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return false, nil, err
	}
	if bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		// Confirm PNG.
		r := bytes.NewReader(data)
		_, err := png.DecodeConfig(r)
		if err == nil {
			textChunks := extractPNGTextChunks(data)
			for k, v := range textChunks {
				results[k] = v
				if containsAIMetadata(v) || containsAIMetadata(k) {
					found = true
				}
			}
			if found {
				return found, results, nil
			}
		}
	}

	// 3. XMP metadata
	if idx := bytes.Index(data, []byte("<x:xmpmeta")); idx != -1 {
		if end := bytes.Index(data[idx:], []byte("</x:xmpmeta>")); end != -1 {
			xmlBlock := data[idx : idx+end+len("</x:xmpmeta>")]
			results["XMP"] = string(xmlBlock)
			if containsAIMetadata(string(xmlBlock)) {
				found = true
			}

			if found {
				return found, results, nil
			}
		}
	}

	return false, nil, nil
}

var aiMetadataKeywords = []*regexp.Regexp{
	regexp.MustCompile(`(?i)Stable Diffusion`),
	regexp.MustCompile(`(?i)MidJourney`),
	regexp.MustCompile(`(?i)DALL-E`),

	regexp.MustCompile(`(?i)^prompt`),
	regexp.MustCompile(`(?i)^parameters`),
	regexp.MustCompile(`(?i)^negative_prompt`),
	regexp.MustCompile(`(?i)^sampler`),
	regexp.MustCompile(`(?i)^seed`),
	regexp.MustCompile(`(?i)^cfg_scale`),

	// Stable Diffusion was observed to have a `parameters:` with
	// a bunch of sub-key/value headers inside.
	regexp.MustCompile(`(?i)steps:`),
	regexp.MustCompile(`(?i)sampler:`),
	regexp.MustCompile(`(?i)seed:`),
	regexp.MustCompile(`(?i)model:`), // Top-level Model: could be e.g. camera model.
	regexp.MustCompile(`(?i)cfg scale:`),
}

func containsAIMetadata(s string) bool {
	for _, kw := range aiMetadataKeywords {
		if kw.MatchString(s) {
			return true
		}
	}
	return false
}

// extractPNGTextChunks manually parses PNG tEXt/zTXt/iTXt chunks
func extractPNGTextChunks(data []byte) map[string]string {
	results := make(map[string]string)

	const pngHeaderSize = 8
	if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		return results
	}
	offset := pngHeaderSize

	for offset < len(data) {
		if offset+8 > len(data) {
			break
		}
		length := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		chunkType := string(data[offset+4 : offset+8])
		if offset+8+length > len(data) {
			break
		}
		chunkData := data[offset+8 : offset+8+length]
		offset += 12 + length // length + type(4) + CRC(4)

		if chunkType == "tEXt" || chunkType == "iTXt" {
			parts := bytes.SplitN(chunkData, []byte{0}, 2)
			if len(parts) == 2 {
				key := string(parts[0])
				val := string(parts[1])
				results[key] = val
			}
		}
	}

	return results
}
