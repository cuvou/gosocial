package photo

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/cuvou/gosocial/pkg/log"
)

// GifToMP4 converts the image into an mp4 gifv. Requires the `ffmpeg` program to be installed, or returns an error.
func GifToMP4(filename string, cfg *UploadConfig) error {
	var (
		gifSize int64
		mp4Size int64
	)

	// Write GIF to temp file
	fh, err := os.CreateTemp("", "gosocial-*.gif")
	if err != nil {
		return err
	}
	defer os.Remove(fh.Name())

	if n, err := fh.Write(cfg.Data); err != nil {
		return err
	} else {
		gifSize = int64(n)
		log.Debug("GifToMP4: written %d bytes to %s", gifSize, fh.Name())
	}

	// Prepare an mp4 tempfile to write to
	var mp4 = strings.TrimSuffix(fh.Name(), ".gif") + ".mp4"

	// Run ffmpeg
	command := []string{
		"ffmpeg",
		"-i", fh.Name(), // .gif name
		"-movflags", "faststart",
		"-pix_fmt", "yuv420p",
		"-vf", `scale=trunc(iw/2)*2:trunc(ih/2)*2`,
		mp4, // .mp4 name
	}
	log.Debug("GifToMP4: Run command: %s", command)
	cmd := exec.Command(command[0], command[1:]...)
	if stdoutErr, err := cmd.CombinedOutput(); err != nil {
		log.Error("ffmpeg failed:\n%s", stdoutErr)

		return fmt.Errorf("GIF conversion didn't work (ffmpeg might not be installed): %s", err)
	}

	// Make sure the output file isn't empty.
	if stat, err := os.Stat(mp4); !os.IsNotExist(err) {
		mp4Size = stat.Size()
		log.Debug("GifToMP4: stats of generated file %s: %d bytes", mp4, mp4Size)
		if stat.Size() == 0 {
			return errors.New("GIF conversion failed: output mp4 file was empty")
		}
	}

	// Place the .mp4 (not .gif) in the static/photos/ folder
	if !strings.HasSuffix(filename, ".mp4") {
		filename = strings.TrimSuffix(filename, ".gif") + ".mp4"
	}
	if path, err := EnsurePath(filename); err == nil {
		// Copy the mp4 tempfile into the right place
		srcFile, err := ioutil.ReadFile(mp4)
		if err != nil {
			return err
		}

		destFile, err := os.Create(path)
		if err != nil {
			return err
		}
		defer destFile.Close()

		w, err := destFile.Write(srcFile)

		log.Debug("GifToMP4: Copy tempfile %s => %s; w=%d err=%s", mp4, path, w, err)
	}

	log.Info("GifToMP4: converted GIF (%d bytes, %f MB) to MP4 (%d bytes, %f MB) for a %f%% savings",
		gifSize, float64(gifSize)/1024/1024,
		mp4Size, float64(mp4Size)/1024/1024,
		float64(gifSize)/float64(mp4Size),
	)

	return nil
}
