// internkim-maild answers mail operations for whoever's account the call carries.
// It listens on loopback only, holds nothing between calls, and keeps no credential:
// a password the call carries sealed is opened with the box key this computer holds.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"gitlab.com/eastriver/internkim/internal/mail"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func main() {
	listenAddress := flag.String("listen", "127.0.0.1:18092", "loopback address to answer on")
	boxStatePath := flag.String("box-state", blueclaw.CompanyHostBoxStatePath, "the box's key and company, which open a sealed mail password")
	flag.Parse()

	server := &http.Server{
		Addr:              *listenAddress,
		Handler:           mail.Handler(mail.StandardBackend{}, mail.BoxPasswords(*boxStatePath)),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("mail answers on %s", *listenAddress)
	if errorValue := server.ListenAndServe(); errorValue != nil {
		log.Fatalf("mail stopped: %v", errorValue)
	}
}
