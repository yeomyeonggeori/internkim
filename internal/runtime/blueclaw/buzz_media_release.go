package blueclaw

import "fmt"

// MediaServerRelease is one architecture's pinned versitygw tarball.
type MediaServerRelease struct {
	Machine   string
	AssetName string
	SHA256    string
	URL       string
}

const buzzMediaReleaseBaseURL = "https://github.com/versity/versitygw/releases/download"

var buzzMediaAssetSHA256ByMachine = map[string]string{
	"aarch64": "b34051d33f5a9c457f790896acb7bd7d7e15ad8d92efb70616b924f37e401910",
	"x86_64":  "2ba2c734d10d2c4e651d03182cb4b246656bc735a2f282db7b0b73fba6073467",
}

var buzzMediaAssetTargetByMachine = map[string]string{
	"aarch64": "arm64",
	"x86_64":  "x86_64",
}

// BuzzMediaReleaseFor resolves what `uname -m` reports to the tarball this
// release installs. Missing means the board is an architecture versity does
// not publish, which the caller reports instead of installing nothing.
func BuzzMediaReleaseFor(machine string) (MediaServerRelease, bool) {
	target, isPublished := buzzMediaAssetTargetByMachine[machine]
	if !isPublished {
		return MediaServerRelease{}, false
	}
	assetName := fmt.Sprintf("versitygw_v%s_Linux_%s.tar.gz", BuzzMediaVersion, target)
	return MediaServerRelease{
		Machine:   machine,
		AssetName: assetName,
		SHA256:    buzzMediaAssetSHA256ByMachine[machine],
		URL:       fmt.Sprintf("%s/v%s/%s", buzzMediaReleaseBaseURL, BuzzMediaVersion, assetName),
	}, true
}

// BuzzMediaBucketPath is the directory the posix backend serves as the bucket.
func BuzzMediaBucketPath() string {
	return BuzzMediaRootPath + "/" + BuzzMediaBucket
}

// BuzzMediaPublishedMachines lists the architectures this release pins, for
// error messages that have to say what was expected.
func BuzzMediaPublishedMachines() []string {
	return []string{"aarch64", "x86_64"}
}
