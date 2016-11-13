package resize

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strconv"
	"strings"

	resizer "github.com/nfnt/resize"
)

func Resize(name string, f io.Reader, r *http.Request) error {
	swi := r.URL.Query().Get("w")
	lname := strings.ToLower(name)
	if (strings.HasSuffix(lname, ".jpg") || strings.HasSuffix(lname, ".png") || strings.HasSuffix(lname, ".gif")) && swi != "" {
		wi, err := strconv.Atoi(swi)
		if err != nil {
			return err
		}
		img, format, err := image.Decode(f)
		if err != nil {
			return err
		}

		// resize to width `wi` using Lanczos resampling
		// and preserve aspect ratio
		m := resizer.Resize(uint(wi), 0, img, resizer.Lanczos3)
		b := &bytes.Buffer{}

		switch format {
		case "jpeg":
			if err := jpeg.Encode(b, m, nil); err != nil {
				return err
			}
		case "gif":
			if err := gif.Encode(b, m, nil); err != nil {
				return err
			}

		case "png":
			if err := png.Encode(b, m); err != nil {
				return err
			}

		}
		f = bytes.NewReader(b.Bytes())
	}
	return nil
}