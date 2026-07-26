package media

import (
	"encoding/binary"
	"image"
)

// readExifOrientation extracts the EXIF Orientation tag (0x0112) from JPEG
// bytes, returning a value in 1..8. It returns 1 (normal) when the image is not
// a JPEG, carries no EXIF, or the tag is absent, so callers can apply the
// transform unconditionally.
func readExifOrientation(content []byte) int {
	segment := jpegExifSegment(content)
	if segment == nil {
		return 1
	}
	return exifOrientationFromTIFF(segment)
}

func jpegExifSegment(content []byte) []byte {
	if len(content) < 4 || content[0] != 0xFF || content[1] != 0xD8 {
		return nil
	}
	offset := 2
	for offset+4 <= len(content) {
		if content[offset] != 0xFF {
			return nil
		}
		marker := content[offset+1]
		if marker == 0xDA || marker == 0xD9 {
			return nil
		}
		segmentLength := int(binary.BigEndian.Uint16(content[offset+2 : offset+4]))
		if segmentLength < 2 || offset+2+segmentLength > len(content) {
			return nil
		}
		payload := content[offset+4 : offset+2+segmentLength]
		if marker == 0xE1 && len(payload) >= 6 && string(payload[:4]) == "Exif" {
			return payload[6:]
		}
		offset += 2 + segmentLength
	}
	return nil
}

func exifOrientationFromTIFF(tiff []byte) int {
	if len(tiff) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	firstIFD := int(order.Uint32(tiff[4:8]))
	if firstIFD+2 > len(tiff) {
		return 1
	}
	entryCount := int(order.Uint16(tiff[firstIFD : firstIFD+2]))
	entriesStart := firstIFD + 2
	for index := 0; index < entryCount; index++ {
		entryOffset := entriesStart + index*12
		if entryOffset+12 > len(tiff) {
			return 1
		}
		tag := order.Uint16(tiff[entryOffset : entryOffset+2])
		if tag != 0x0112 {
			continue
		}
		value := int(order.Uint16(tiff[entryOffset+8 : entryOffset+10]))
		if value >= 1 && value <= 8 {
			return value
		}
		return 1
	}
	return 1
}

// applyOrientation bakes an EXIF orientation into the pixels so the image
// displays correctly after metadata is stripped.
func applyOrientation(source image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return source
	}
	bounds := source.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	swapsAxes := orientation >= 5
	targetWidth, targetHeight := width, height
	if swapsAxes {
		targetWidth, targetHeight = height, width
	}
	target := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			targetX, targetY := orientedPoint(orientation, x, y, width, height)
			target.Set(targetX, targetY, source.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}
	return target
}

func orientedPoint(orientation, x, y, width, height int) (int, int) {
	switch orientation {
	case 2:
		return width - 1 - x, y
	case 3:
		return width - 1 - x, height - 1 - y
	case 4:
		return x, height - 1 - y
	case 5:
		return y, x
	case 6:
		return height - 1 - y, x
	case 7:
		return height - 1 - y, width - 1 - x
	case 8:
		return y, width - 1 - x
	default:
		return x, y
	}
}
