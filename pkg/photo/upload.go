package photo

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cuvou/gosocial/pkg/cloudflare"
	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/disintegration/imaging"
	"github.com/edwvee/exiffix"
	"github.com/kolesa-team/go-webp/decoder"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
	"golang.org/x/image/draw"
)

type UploadConfig struct {
	User      *models.User
	Extension string // like '.jpg'
	Data      []byte
	Crop      []int // x, y, w, h

	// Special case to force the output extension.
	// e.g.: cert photos will still save as JPEG instead of WebP.
	OutputExtension string // like '.jpg'
}

// UploadPhoto handles an incoming photo to add to a user's account.
//
// Returns:
// - NewFilename() of the created photo file on disk.
// - NewFilename() of the cropped version, or "" if not cropping.
// - error on errors
func UploadPhoto(cfg UploadConfig) (string, string, error) {

	// What will the output file extension be?
	var defaultDBExtension = ".jpg"
	if config.PhotoGallerySaveAsWebP {
		defaultDBExtension = ".webp"
	}

	// Validate and normalize the extension.
	var (
		extension   = strings.ToLower(cfg.Extension)
		dbExtension = extension
	)
	switch extension {
	case ".jpg", ".jpe", ".jpeg":
		extension = ".jpg"
		dbExtension = defaultDBExtension
	case ".png":
		extension = ".png"
		dbExtension = defaultDBExtension
	case ".webp":
		extension = ".webp"
		dbExtension = ".webp"
	case ".gif":
		extension = ".gif"
		dbExtension = ".mp4" // GIF is converted into small mp4 video
	default:
		return "", "", errors.New("unsupported image extension: must be jpg, png, gif, or webp")
	}

	// A forced output extension? e.g. so cert photos will save as JPEG.
	if cfg.OutputExtension != "" {
		dbExtension = cfg.OutputExtension
	}

	// Decide on a filename for this photo.
	var (
		filename     = NewFilename(dbExtension)
		cropFilename = NewFilename(dbExtension)
	)

	// Decode the image using exiffix, which will auto-rotate jpeg images
	// based on their EXIF tags.
	reader := bytes.NewReader(cfg.Data)
	origImage, _, err := exiffix.Decode(reader)
	if err != nil {
		return "", "", err
	}

	// Read the config to get the image width.
	reader.Seek(0, io.SeekStart)
	var width, height = origImage.Bounds().Max.X, origImage.Bounds().Max.Y

	// Find the longest edge, if it's too large (over 1280px)
	// cap it to the max and scale the other dimension proportionally.
	log.Debug("UploadPhoto: taking a w=%d by h=%d image to name it %s", width, height, filename)
	if width >= height {
		log.Debug("Its width(%d) is >= its height (%d)", width, height)
		if width > config.MaxPhotoWidth {
			newWidth := config.MaxPhotoWidth
			log.Debug("\tnewWidth=%d", newWidth)
			log.Debug("\tnewHeight=(%d / %d) * %d", width, height, newWidth)
			height = int((float64(height) / float64(width)) * float64(newWidth))
			width = newWidth
			log.Debug("Its longest is width, scale to %dx%d", width, height)
		}
	} else {
		if height > config.MaxPhotoWidth {
			newHeight := config.MaxPhotoWidth
			width = int((float64(width) / float64(height)) * float64(newHeight))
			height = newHeight
			log.Debug("Its longest is height, scale to %dx%d", width, height)
		}
	}

	// Scale the image.
	scaledImg := Scale(origImage, image.Rect(0, 0, width, height), draw.ApproxBiLinear)

	// Write the image to disk.
	if err := ToDisk(filename, dbExtension, scaledImg, &cfg); err != nil {
		return "", "", err
	}

	// Are we producing a cropped image, too?
	if len(cfg.Crop) >= 4 {
		log.Debug("Also cropping this image to %+v", cfg.Crop)
		var (
			x = cfg.Crop[0]
			y = cfg.Crop[1]
			w = cfg.Crop[2]
			h = cfg.Crop[3]
		)
		croppedImg, err := Crop(origImage, x, y, w, h)
		if err != nil {
			// Error during the crop: return it and just the original image filename
			log.Error("Couldn't crop new profile photo: %s", err)
			return filename, "", nil
		}

		// Scale profile photos down into consistent sizes.
		croppedImg = Scale(croppedImg, image.Rect(0, 0, config.ProfilePhotoWidth, config.ProfilePhotoWidth), draw.ApproxBiLinear)

		// Write that to disk, too.
		log.Debug("Writing cropped image to disk: %s", cropFilename)
		if err := ToDisk(cropFilename, dbExtension, croppedImg, &cfg); err != nil {
			log.Error("Couldn't write cropped photo to disk: %s", err)
			return filename, "", nil
		}

		// Return both filenames!
		return filename, cropFilename, nil
	}

	// Not cropping, return only the first filename.
	return filename, "", nil
}

// Scale down an image. Example:
//
// scaled := Scale(src, image.Rect(0, 0, 200, 200), draw.ApproxBiLinear)
func Scale(src image.Image, rect image.Rectangle, scale draw.Scaler) image.Image {
	dst := image.NewRGBA(rect)
	copyRect := image.Rect(
		rect.Min.X,
		rect.Min.Y,
		rect.Min.X+rect.Max.X,
		rect.Min.Y+rect.Max.Y,
	)
	scale.Scale(dst, copyRect, src, src.Bounds(), draw.Over, nil)
	return dst
}

// Crop an image, returning the new image. Example:
//
// cropped := Crop()
func Crop(src image.Image, x, y, w, h int) (image.Image, error) {
	// Sanity check the crop constraints, e.g. sometimes the front-end might send "203 -1 738 738" with a negative x/y value
	if x < 0 {
		log.Debug("Crop(%d, %d, %d, %d): x value %d too low, cap to zero", x, y, w, h, x)
		x = 0
	}
	if y < 0 {
		log.Debug("Crop(%d, %d, %d, %d): y value %d too low, cap to zero", x, y, w, h, y)
		y = 0
	}
	if x+w > src.Bounds().Dx() {
		log.Debug("Crop(%d, %d, %d, %d): width is too wide", x, y, w, h)
		w = src.Bounds().Dx() - x
	}
	if y+h > src.Bounds().Dy() {
		log.Debug("Crop(%d, %d, %d, %d): height is too tall", x, y, w, h)
		h = src.Bounds().Dy() - y
	}

	// If they are trying to crop a 0x0 image, return an error.
	if w == 0 || h == 0 {
		return nil, errors.New("can't crop to a 0x0 resolution image")
	}

	log.Debug("Crop(): running draw.Copy with dimensions %d, %d, %d, %d", x, y, w, h)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	srcrect := image.Rect(x, y, x+w, y+h)
	draw.Copy(dst, image.Point{}, src, srcrect, draw.Over, nil)
	return dst, nil
}

// Decode an image from disk, returning the Go Image object.
func Decode(filename string) (img image.Image, err error) {
	fh, err := os.Open(DiskPath(filename))
	if err != nil {
		return
	}

	// Decode the image.
	switch filepath.Ext(filename) {
	case ".jpg", ".jpeg", ".jpe":
		// NOTE: new uploads enforce .jpg extension, some legacy pics may have slipped thru
		img, err = jpeg.Decode(fh)
		if err != nil {
			return
		}
	case ".png":
		img, err = png.Decode(fh)
		if err != nil {
			return
		}
	case ".webp":
		img, err = webp.Decode(fh, &decoder.Options{})
		if err != nil {
			return
		}
	default:
		return nil, errors.New("unsupported file type")
	}

	return
}

// Rotate an image, changing its orientation after upload.
//
// Give it your Photo model and it will rotate the Filename and CroppedFilename (if set).
// To bust the browser caches, a new Filename is generated for the rotated images and it
// will be set on your Photo model.
//
// For safety, the Photo will be Saved in the database after its filenames have been updated.
//
// Acceptable rotation degrees are: 90, 180, 270.
func Rotate(photo *models.Photo, rotation int) error {

	// Note: to bust the browser cache, the filenames will need to be changed.
	newFilename, err := rotatePhoto(photo.Filename, rotation)
	if err != nil {
		return err
	}
	photo.Filename = newFilename

	// If there is a cropped filename, rotate it too.
	if photo.CroppedFilename != "" {
		newCroppedFilename, err := rotatePhoto(photo.CroppedFilename, rotation)
		if err != nil {
			return err
		}
		photo.CroppedFilename = newCroppedFilename
	}

	return photo.Save()
}

// Inner function to rotate a photo from filename on disk. Returns the new filename if successful.
func rotatePhoto(filename string, rotation int) (string, error) {
	var (
		ext         = filepath.Ext(filename)
		newImage    image.Image
		newFilename = NewFilename(ext)
	)
	// Decode the image.
	img, err := Decode(filename)
	if err != nil {
		return "", err
	}

	// Rotate the image.
	// NOTE: the imaging module rotates images counterclockwise, so we invert the rotation given.
	switch rotation {
	case 90:
		newImage = imaging.Rotate270(img)
	case 180:
		newImage = imaging.Rotate180(img)
	case 270:
		newImage = imaging.Rotate90(img)
	default:
		return "", errors.New("unsupported degree of rotation, must be one of: 90, 180, 270")
	}

	// Save the new (rotated) image.
	if err := ToDisk(newFilename, ext, newImage, nil); err != nil {
		return "", err
	}

	// Delete the old filename.
	if err := Delete(filename); err != nil {
		return "", fmt.Errorf("deleting old filename (%s): %s", filename, err)
	}

	return newFilename, nil
}

// ReCrop an image, loading the original image from disk. Returns the newly created filename.
func ReCrop(filename string, x, y, w, h int) (string, error) {
	var (
		ext          = filepath.Ext(filename)
		cropFilename = NewFilename(ext)
	)

	// Decode the image.
	img, err := Decode(filename)
	if err != nil {
		return "", err
	}

	// Crop it.
	croppedImg, err := Crop(img, x, y, w, h)
	if err != nil {
		return "", err
	}

	// Scale profile photos down into consistent sizes.
	croppedImg = Scale(croppedImg, image.Rect(0, 0, config.ProfilePhotoWidth, config.ProfilePhotoWidth), draw.ApproxBiLinear)

	// Write it.
	err = ToDisk(cropFilename, ext, croppedImg, nil)
	return cropFilename, err
}

// ParseCropCoords splits a string of x,y,w,h values into proper crop coordinates, or nil.
func ParseCropCoords(coords string) []int {
	// Parse and validate crop coordinates.
	var crop []int
	if len(coords) > 0 {
		aints := strings.Split(coords, ",")
		if len(aints) >= 4 {
			crop = []int{}
			for i, aint := range aints {
				if number, err := strconv.Atoi(strings.TrimSpace(aint)); err == nil {
					crop = append(crop, number)
				} else {
					log.Error("Failure to parse crop coordinates ('%s') at number %d: %s", coords, i, err)
				}
			}
		}
	}

	return crop
}

// ToDisk commits a photo image to disk in the right file format.
//
// Filename is like NewFilename() and it goes to e.g. "./web/static/photos/"
//
// Encoding rules:
// - JPEG and PNG uploads are saved as JPEG
// - GIF uploads are transmuted to MP4
func ToDisk(filename string, extension string, img image.Image, cfg *UploadConfig) error {
	// GIF path handled specially (note: it will come in with extension=".mp4")
	if extension == ".gif" || extension == ".mp4" {
		// Requires an upload config (ReCrop not supported)
		if cfg == nil {
			return errors.New("can't update your GIF after original upload")
		}
		return GifToMP4(filename, cfg)
	}

	if path, err := EnsurePath(filename); err == nil {
		fh, err := os.Create(path)
		if err != nil {
			return err
		}
		defer fh.Close()

		switch extension {
		case ".jpg", ".jpe", ".jpeg", ".png":
			// NOTE: new uploads enforce .jpg extension always, some legacy pics (.png too) may have slipped thru
			jpeg.Encode(fh, img, &jpeg.Options{
				Quality: config.JpegQuality,
			})
		case ".webp":
			options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, config.WebPCompressionFactor)
			if err != nil {
				return err
			}

			if err := webp.Encode(fh, img, options); err != nil {
				return err
			}
		}
	} else {
		return fmt.Errorf("couldn't EnsurePath: %s", err)
	}

	return nil
}

// Delete a photo from disk.
func Delete(filename string) error {
	if len(filename) > 0 {
		err := os.Remove(DiskPath(filename))
		if err != nil {
			return fmt.Errorf("os.Remove('%s'): %w", filename, err)
		}

		// Purge from Cloudflare CDN too, log errors.
		err = PurgePhotoURL(filename)
		if err != nil {
			log.Error("photo.Delete: PurgePhotoURL: %s", err)
		}

		return nil
	}
	return errors.New("filename is required")
}

// PurgePhotoURL sends a Cloudflare purge command for a (deleted) photo to remove it from the CDN immediately.
func PurgePhotoURL(filename string) error {
	return cloudflare.PurgeURL(
		[]string{
			strings.TrimSuffix(config.Current.BaseURL, "/") + URLPath(filename),
		},
	)
}
