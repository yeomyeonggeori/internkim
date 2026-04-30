//go:build !linux

package capabilityd

import (
	"errors"
	"net"
)

func listenVSock(port int) (net.Listener, error) {
	return nil, errors.New("capabilityd vsock listener is only available on linux")
}
