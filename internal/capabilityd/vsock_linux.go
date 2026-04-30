//go:build linux

package capabilityd

import (
	"net"

	"github.com/mdlayher/vsock"
)

func listenVSock(port int) (net.Listener, error) {
	return vsock.Listen(uint32(port), nil)
}
