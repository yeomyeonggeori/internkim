package box

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const pairingPageReadTimeout = 5 * time.Second

type pairingPage struct {
	server    *http.Server
	hostName  string
	addresses []string
}

func (daemon Daemon) openPairingPage(identity Identity) *pairingPage {
	if daemon.Places.PairingPageListenAddress == "" {
		return nil
	}
	listener, errorValue := net.Listen("tcp", daemon.Places.PairingPageListenAddress)
	if errorValue != nil {
		log.Printf("the pairing code is not shown on this network: %v", errorValue)
		return nil
	}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	hostName := localHostName()
	page := &pairingPage{
		server:    &http.Server{Handler: daemon.pairingPageHandler(identity), ReadHeaderTimeout: pairingPageReadTimeout},
		hostName:  hostName,
		addresses: localPageAddresses(hostName, lanAddress(), port),
	}
	go func() {
		if errorValue := page.server.Serve(listener); !errors.Is(errorValue, http.ErrServerClosed) {
			log.Printf("the pairing page stopped: %v", errorValue)
		}
	}()
	log.Printf("the pairing code is shown on this network at %s", strings.Join(page.addresses, " and "))
	return page
}

func (page *pairingPage) close() {
	if page == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), pairingPageReadTimeout)
	defer cancel()
	page.server.Shutdown(ctx)
}

func (page *pairingPage) localPage() LocalPage {
	if page == nil {
		return LocalPage{}
	}
	return LocalPage{HostName: page.hostName, Addresses: page.addresses}
}

func (daemon Daemon) pairingPageHandler(identity Identity) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Content-Security-Policy", "default-src 'none'")
		fmt.Fprint(writer, pairingPageDocument(fingerprintOf(identity.PublicKey()), daemon.shownCode()))
	})
}

func (daemon Daemon) shownCode() *PairingCode {
	shown, isLive, errorValue := ShownPairingCode(daemon.Places.StateDirectoryPath, daemon.now())
	if errorValue != nil || !isLive {
		return nil
	}
	return &shown
}

func pairingPageDocument(fingerprint string, shown *PairingCode) string {
	body := "<p>This box shows no code right now. A new one arrives within a minute; reload this page.</p>"
	if shown != nil {
		body = fmt.Sprintf("<p><strong>%s</strong></p><p>Enter it on the company setup page before %s.</p>",
			html.EscapeString(shown.Code), html.EscapeString(shown.ExpiresAt.Local().Format("15:04")))
	}
	return "<!doctype html><meta charset=utf-8><meta name=viewport content=\"width=device-width\">" +
		"<title>internkim box</title><p>Box …" + html.EscapeString(fingerprint) + "</p>" + body
}

func fingerprintOf(publicKey string) string {
	return publicKey[max(len(publicKey)-4, 0):]
}

func localHostName() string {
	name, errorValue := os.Hostname()
	if errorValue != nil {
		return ""
	}
	label, _, _ := strings.Cut(strings.ToLower(name), ".")
	return label
}

func lanAddress() string {
	connection, errorValue := net.Dial("udp4", "192.0.2.1:9")
	if errorValue != nil {
		return ""
	}
	defer connection.Close()
	address, isUDP := connection.LocalAddr().(*net.UDPAddr)
	if !isUDP || !address.IP.IsPrivate() {
		return ""
	}
	return address.IP.String()
}

func localPageAddresses(hostName, lanAddress, port string) []string {
	var addresses []string
	if hostName != "" {
		addresses = append(addresses, "http://"+hostName+".local:"+port+"/")
	}
	if lanAddress != "" {
		addresses = append(addresses, "http://"+lanAddress+":"+port+"/")
	}
	return addresses
}
