package companyhost

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
)

var ErrNoFileJournal = errors.New("the company keeps no file journal to replay")

type FileJournalApplier func(isFull bool, removed []string, contents io.Reader) error

type FileJournalReplay func(apply FileJournalApplier) (int, error)

func replayTheFileJournal(replay FileJournalReplay, destinations map[string]string, extracted *hostbackup.ExtractionReport, progress io.Writer) error {
	replayed, errorValue := replay(func(isFull bool, removed []string, contents io.Reader) error {
		if isFull {
			for _, rootPath := range destinations {
				if errorValue := emptyTheDirectory(rootPath); errorValue != nil {
					return errorValue
				}
			}
		}
		for _, archivedName := range removed {
			if errorValue := removeArchived(destinations, archivedName); errorValue != nil {
				return errorValue
			}
		}
		report, errorValue := hostbackup.ExtractFiles(contents, destinations)
		if errorValue != nil {
			return errorValue
		}
		extracted.Files += report.Files
		extracted.Bytes += report.Bytes
		extracted.Unresolved = append(extracted.Unresolved, report.Unresolved...)
		return nil
	})
	if errors.Is(errorValue, ErrNoFileJournal) {
		fmt.Fprintln(progress, "  the company keeps no file journal yet, so the files are the snapshot's")
		return nil
	}
	if errorValue != nil {
		return fmt.Errorf("replaying the company's file journal over the snapshot: %w", errorValue)
	}
	fmt.Fprintf(progress, "  replayed %d entries of the company's file journal over the snapshot\n", replayed)
	return nil
}

func removeArchived(destinations map[string]string, archivedName string) error {
	role, relative, isNamed := strings.Cut(archivedName, "/")
	rootPath, isKnown := destinations[role]
	if !isNamed || !isKnown {
		return fmt.Errorf("the file journal removes %q, which names no root this restore writes", archivedName)
	}
	cleaned := filepath.Clean(filepath.FromSlash(relative))
	if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("the file journal removes %q, which lies outside its root", archivedName)
	}
	return os.RemoveAll(filepath.Join(rootPath, cleaned))
}
