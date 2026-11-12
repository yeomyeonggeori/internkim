package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"

	"github.com/yeomyeonggeori/internkim/internal/filestream"
	"github.com/yeomyeonggeori/internkim/internal/mediagate"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func runMediaGate(arguments []string) error {
	flags := flag.NewFlagSet("media-gate", flag.ContinueOnError)
	fileJournalSocketPath := flags.String("file-journal-socket", "", "ask the file journal at this socket to keep every stored attachment")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("media-gate takes no arguments, only -file-journal-socket")
	}
	upstream, errorValue := url.Parse("http://" + blueclaw.BuzzMediaServerAddress)
	if errorValue != nil {
		return errorValue
	}
	return http.ListenAndServe(blueclaw.BuzzMediaAddress, mediaGate(upstream, *fileJournalSocketPath).Handler())
}

func mediaGate(upstream *url.URL, fileJournalSocketPath string) mediagate.Gate {
	gate := mediagate.Gate{Upstream: upstream}
	if fileJournalSocketPath != "" {
		gate.KeepFiles = func(ctx context.Context) error {
			return filestream.AskToKeep(ctx, fileJournalSocketPath)
		}
	}
	return gate
}
