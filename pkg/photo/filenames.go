package photo

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/google/uuid"
)

// Functions that deal with giving photos their:
// - Filename
// - URL prefix (/static/photos or maybe CDN?)

/*
NewFilename generates a Filename with an extension (".jpg").

The filename is a random UUID string, with a couple of directory
paths in front consisting of the first few characters (to keep
directory sizes under control over time). Example:

"91/b9/91b908db-4007-41b2-bbca-71a6526e59aa.jpg"
*/
func NewFilename(ext string) string {
	basename := uuid.New().String()
	first2 := basename[:2]
	next2 := basename[2:4]
	log.Debug("photo.NewFilename: UUID %s first2 %s next2 %s", basename, first2, next2)
	return fmt.Sprintf(
		"%s/%s/%s%s",
		first2, next2, basename, ext,
	)
}

// DiskPath returns the local disk path to a photo Filename.
func DiskPath(filename string) string {
	return config.PhotoDiskPath + "/" + filename
}

// URLPath returns the public HTTP path to a photo. May be relative like "/static/photos" or could be a full CDN.
func URLPath(filename string) string {
	return config.PhotoWebPath + "/" + filename
}

/*
EnsurePath makes sure the local './web/static/photos/' path is ready
to write an image to, taking into account path parameters in the
image filename.

The filename is like from NewFilename(), just the photo Filename portion.
It is appended to the PhotoDiskPath.

Returns the full path ("./web/static/photos/...") ready for the caller
to use it for writing.
*/
func EnsurePath(filename string) (string, error) {
	fullpath := DiskPath(filename)
	dir := filepath.Dir(fullpath)
	log.Debug("photo.EnsurePath: check that %s exists", dir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fullpath, fmt.Errorf("EnsurePath: %s", err)
	} else {
		return fullpath, nil
	}
}
