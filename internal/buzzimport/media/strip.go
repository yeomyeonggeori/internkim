package media

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// StripMetadata re-encodes an image so it carries no EXIF, GPS, or ancillary
// metadata, which the relay rejects. It returns the canonical bytes and their
// mime type, or ok=false for formats the standard library cannot decode.
func StripMetadata(content []byte, mimeType string) ([]byte, string, bool) {
	decoded, format, errorValue := image.Decode(bytes.NewReader(content))
	if errorValue != nil {
		return nil, "", false
	}
	decoded = applyOrientation(decoded, readExifOrientation(content))
	var buffer bytes.Buffer
	switch format {
	case "png":
		if errorValue := (&png.Encoder{CompressionLevel: png.DefaultCompression}).Encode(&buffer, decoded); errorValue != nil {
			return nil, "", false
		}
		return buffer.Bytes(), "image/png", true
	case "gif":
		if errorValue := gif.Encode(&buffer, decoded, nil); errorValue != nil {
			return nil, "", false
		}
		return buffer.Bytes(), "image/gif", true
	default:
		if errorValue := jpeg.Encode(&buffer, decoded, &jpeg.Options{Quality: 90}); errorValue != nil {
			return nil, "", false
		}
		return buffer.Bytes(), "image/jpeg", true
	}
}
