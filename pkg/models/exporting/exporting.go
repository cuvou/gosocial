package exporting

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/photo"
)

// ExportUser creates a data export archive of ALL data stored about a user account..
func ExportUser(user *models.User, filename string) error {
	if !strings.HasSuffix(filename, ".zip") {
		return errors.New("output file should be a .zip file")
	}

	// Prepare the output zip writer.
	fh, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("creating output file (%s): %s", filename, err)
	}

	zw := zip.NewWriter(fh)
	defer zw.Close()

	// Export all their database tables into the zip.
	return ExportModels(zw, user)
}

// ZipJson serializes a JSON file into the zipfile.
func ZipJson(zw *zip.Writer, filename string, v any) error {
	fh, err := zw.Create(filename)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(fh)
	encoder.SetIndent("", " ")

	return encoder.Encode(v)
}

// ZipPhoto copies a user photo into the ZIP archive.
func ZipPhoto(zw *zip.Writer, prefix, filename string) error {
	var (
		diskPath  = photo.DiskPath(filename)
		data, err = os.ReadFile(diskPath)
	)
	if err != nil {
		if os.IsNotExist(err) {
			// Not fatal but log it.
			log.Error("ZipPhoto(%s): read from disk: %s", diskPath, err)
			return nil
		}
		return fmt.Errorf("ZipPhoto(%s): read from disk: %s", diskPath, err)
	}

	outfh, err := zw.Create(path.Join(prefix, filename))
	if err != nil {
		return fmt.Errorf("ZipPhoto(%s): create in zip: %s", filename, err)
	}

	log.Info("Add photo to zip: %s", filename)

	_, err = outfh.Write(data)
	return err
}

// ZipVideo copies files from the videos folder into the ZIP archive.
//
// Filenames are like "web/static/videos/nonce/thumbnail0.jpg"
func ZipVideo(zw *zip.Writer, prefix string, filenames []string) error {

	for _, filename := range filenames {
		var (
			basename = filepath.Base(filename)
			outname  = path.Join(prefix, basename)
		)

		src, err := os.Open(filename)
		if err != nil {
			log.Error("ZipVideo(%s): read from disk: %s", filename, err)
			continue
		}

		dst, err := zw.Create(outname)
		written, err := io.Copy(dst, src)
		if err != nil {
			log.Error("ZipVideo(%s): couldn't write to zip: %s", filename, err)
		}

		log.Info("Add video to zip: %s (%d bytes)", outname, written)
	}

	return nil
}
