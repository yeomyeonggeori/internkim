package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/deviceexport"
)

const deviceExportStampLayout = "20060102T150405Z"

type importDeviceArguments struct {
	ExportDirectoryPath string
	ConnectionPath      string
	HostPasswdPath      string
	HostGroupPath       string
}

func runImportDevice(arguments []string) error {
	parsed, errorValue := parseImportDeviceArguments(arguments)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := companyhost.RequireAdministrator(); errorValue != nil {
		return errorValue
	}
	takenAt, errorValue := exportStamp(parsed.ExportDirectoryPath)
	if errorValue != nil {
		return errorValue
	}
	archivePath, errorValue := deviceArchivePath(takenAt)
	if errorValue != nil {
		return errorValue
	}
	fmt.Fprintf(os.Stdout, "Writing the device's export as a backup archive at %s…\n", archivePath)
	report, errorValue := deviceexport.Convert(deviceexport.Request{
		ExportDirectoryPath: parsed.ExportDirectoryPath,
		ConnectionPath:      parsed.ConnectionPath,
		HostPasswdPath:      parsed.HostPasswdPath,
		HostGroupPath:       parsed.HostGroupPath,
		ArchivePath:         archivePath,
		CreatedAt:           takenAt,
	})
	if errorValue != nil {
		return fmt.Errorf("the device's export could not be written as an archive, and nothing on this computer changed: %w", errorValue)
	}
	printConversion(os.Stdout, report)
	result, errorValue := companyhost.Restore(companyhost.RestoreRequest{ArchivePath: archivePath, Now: time.Now()}, thisComputer{}, os.Stdout)
	if errorValue != nil {
		return fmt.Errorf("%w\nThe archive stays at %s; `internkim restore %s` runs the restore again", errorValue, archivePath, archivePath)
	}
	printRestore(os.Stdout, result)
	return nil
}

func parseImportDeviceArguments(arguments []string) (importDeviceArguments, error) {
	flags := flag.NewFlagSet("import-device", flag.ContinueOnError)
	parsed := importDeviceArguments{}
	flags.StringVar(&parsed.ConnectionPath, "connection", "", "the company's connection file from company setup")
	flags.StringVar(&parsed.HostPasswdPath, "host-passwd", "/etc/passwd", "the device's accounts, which name the owners of its own files")
	flags.StringVar(&parsed.HostGroupPath, "host-group", "/etc/group", "the device's groups")
	positional := []string{}
	remaining := arguments
	for {
		if errorValue := flags.Parse(remaining); errorValue != nil {
			return importDeviceArguments{}, errorValue
		}
		if flags.NArg() == 0 {
			break
		}
		positional = append(positional, flags.Arg(0))
		remaining = flags.Args()[1:]
	}
	if len(positional) != 1 {
		return importDeviceArguments{}, fmt.Errorf("name the one directory the device's migration export wrote")
	}
	if parsed.ConnectionPath == "" {
		return importDeviceArguments{}, fmt.Errorf("--connection is required: the device never held a connection file, so the restored host takes the one company setup gives")
	}
	parsed.ExportDirectoryPath = positional[0]
	return parsed, nil
}

func exportStamp(exportDirectoryPath string) (time.Time, error) {
	base := filepath.Base(filepath.Clean(exportDirectoryPath))
	stamp := base[strings.LastIndex(base, "-")+1:]
	takenAt, errorValue := time.Parse(deviceExportStampLayout, stamp)
	if errorValue != nil {
		return time.Time{}, fmt.Errorf("%s is not named by the time the export was taken, the way migration-export names it", exportDirectoryPath)
	}
	return takenAt, nil
}

func deviceArchivePath(takenAt time.Time) (string, error) {
	directoryPath := companyhost.DefaultBackupDirectoryPath()
	if errorValue := os.MkdirAll(directoryPath, 0o700); errorValue != nil {
		return "", errorValue
	}
	archivePath := filepath.Join(directoryPath, "internkim-device-"+takenAt.UTC().Format(deviceExportStampLayout)+".tar")
	if _, errorValue := os.Stat(archivePath); !errors.Is(errorValue, fs.ErrNotExist) {
		return "", fmt.Errorf("%s already exists. Restore it with `internkim restore %s`, or move it away to write it again", archivePath, archivePath)
	}
	return archivePath, nil
}

func printConversion(output io.Writer, report deviceexport.Report) {
	fmt.Fprintf(output, "  %s, %d file entries\n", companyhost.DescribeSize(report.Manifest.TotalSize()), report.Entries)
	secrets := append([]string{}, report.CarriedSecrets...)
	sort.Strings(secrets)
	fmt.Fprintf(output, "  carried the device's %s\n", strings.Join(secrets, ", "))
	fmt.Fprintf(output, "  left out of the device's own state: %s\n", strings.Join(report.LeftOut, ", "))
}
