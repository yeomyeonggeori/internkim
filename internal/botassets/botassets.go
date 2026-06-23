package botassets

import _ "embed"

//go:embed internkim.png
var avatarPNG []byte

func AvatarPNG() []byte {
	return avatarPNG
}

func AvatarFileName() string {
	return "internkim.png"
}
