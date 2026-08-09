// internkim-maild answers mail operations for whoever's account the call carries.
// It listens on loopback only, holds nothing between calls, and keeps no credential.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"gitlab.com/eastriver/internkim/internal/mail"
)

func main() {
	listenAddress := flag.String("listen", "127.0.0.1:18092", "loopback address to answer on")
	flag.Parse()

	server := &http.Server{
		Addr:              *listenAddress,
		Handler:           mail.Handler(mail.StandardBackend{}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("mail answers on %s", *listenAddress)
	if errorValue := server.ListenAndServe(); errorValue != nil {
		log.Fatalf("mail stopped: %v", errorValue)
	}
}
