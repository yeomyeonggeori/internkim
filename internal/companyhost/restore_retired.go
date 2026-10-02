package companyhost

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
)

var retiredDumpEntries = []string{
	"EXTENSION - vector",
	"COMMENT - EXTENSION vector",
	"TABLE public memory_fact_embedding",
	"TABLE DATA public memory_fact_embedding",
	"CONSTRAINT public memory_fact_embedding memory_fact_embedding_pkey",
	"FK CONSTRAINT public memory_fact_embedding memory_fact_embedding_fact_id_fkey",
	"INDEX public memory_fact_embedding_hnsw_idx",
	"TABLE public memory_fact_trigger_embedding",
	"TABLE DATA public memory_fact_trigger_embedding",
	"CONSTRAINT public memory_fact_trigger_embedding memory_fact_trigger_embedding_pkey",
	"FK CONSTRAINT public memory_fact_trigger_embedding memory_fact_trigger_embedding_trigger_id_fkey",
}

func restoreWithoutRetiredEntries(platform backupPlatform, archive hostbackup.Archive, machine Machine, database string, member hostbackup.Member, progress io.Writer) error {
	var list string
	if errorValue := archive.ReadMember(member.Name, func(input io.Reader) error {
		listed, errorValue := platform.ListDump(machine, input)
		list = listed
		return errorValue
	}); errorValue != nil {
		return errorValue
	}
	kept, leftOut := withoutRetiredEntries(list)
	if len(leftOut) == 0 {
		return archive.ReadMember(member.Name, func(input io.Reader) error {
			return platform.RestoreDatabase(machine, database, input, "")
		})
	}
	fmt.Fprintf(progress, "  %s carries what the agent no longer keeps, and the restore leaves it out: %s\n", database, strings.Join(leftOut, "; "))
	listPath, errorValue := writeRestoreList(kept)
	if errorValue != nil {
		return errorValue
	}
	defer os.Remove(listPath)
	return archive.ReadMember(member.Name, func(input io.Reader) error {
		return platform.RestoreDatabase(machine, database, input, listPath)
	})
}

func withoutRetiredEntries(list string) (string, []string) {
	kept := []string{}
	leftOut := []string{}
	for _, line := range strings.Split(list, "\n") {
		identity, isRetired := retiredIdentityOf(line)
		if isRetired {
			leftOut = append(leftOut, identity)
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n"), leftOut
}

func retiredIdentityOf(line string) (string, bool) {
	description, isEntry := entryDescription(line)
	if !isEntry {
		return "", false
	}
	for _, identity := range retiredDumpEntries {
		if description == identity || strings.HasPrefix(description, identity+" ") {
			return identity, true
		}
	}
	return "", false
}

func entryDescription(line string) (string, bool) {
	dumpID, afterDumpID, hasDumpID := strings.Cut(line, "; ")
	if !hasDumpID || dumpID == "" || strings.Trim(dumpID, "0123456789") != "" {
		return "", false
	}
	fields := strings.SplitN(afterDumpID, " ", 3)
	if len(fields) != 3 {
		return "", false
	}
	return fields[2], true
}

func writeRestoreList(list string) (string, error) {
	file, errorValue := os.CreateTemp("", "internkim-restore-list-*")
	if errorValue != nil {
		return "", fmt.Errorf("the restore could not write the list of what to restore: %w", errorValue)
	}
	defer file.Close()
	if _, errorValue := file.WriteString(list); errorValue != nil {
		os.Remove(file.Name())
		return "", fmt.Errorf("the restore could not write the list of what to restore: %w", errorValue)
	}
	if errorValue := file.Chmod(0o644); errorValue != nil {
		os.Remove(file.Name())
		return "", fmt.Errorf("the restore could not let the database account read the list of what to restore: %w", errorValue)
	}
	return file.Name(), nil
}
