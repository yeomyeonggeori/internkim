package aptrepository

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
)

// readXZ decompresses an xz member of a .deb.
//
// The standard library has no xz reader, and dpkg-deb compresses control.tar
// with xz by default, so a package built by Debian tooling rather than by nfpm
// arrives this way. Shelling out keeps the dependency list as it is; the error
// names the missing program rather than reporting the package as malformed.
func readXZ(body []byte) ([]byte, error) {
	command := exec.Command("xz", "--decompress", "--stdout")
	command.Stdin = bytes.NewReader(body)
	var decompressed, spoken bytes.Buffer
	command.Stdout = &decompressed
	command.Stderr = &spoken
	if errorValue := command.Run(); errorValue != nil {
		if errors.Is(errorValue, exec.ErrNotFound) {
			return nil, errors.New("this package's control member is xz-compressed and `xz` is not on PATH; install xz-utils, or build the package with gzip")
		}
		return nil, fmt.Errorf("xz could not decompress the control member: %s", spoken.String())
	}
	return decompressed.Bytes(), nil
}
