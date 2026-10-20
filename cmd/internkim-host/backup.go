package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func runBackup(arguments []string) error {
	flags := flag.NewFlagSet("backup", flag.ContinueOnError)
	directoryPath := flags.String("directory", companyhost.DefaultBackupDirectoryPath(), "where the archive is written")
	keep := flags.Int("keep", 0, "after writing, keep only this many of the newest backups in the directory; 0 keeps them all")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("backup takes no arguments, only --directory and --keep")
	}
	if errorValue := companyhost.RequireAdministrator(); errorValue != nil {
		return errorValue
	}
	result, errorValue := companyhost.Backup(companyhost.BackupRequest{
		DirectoryPath: *directoryPath,
		Keep:          *keep,
		Now:           time.Now(),
	}, thisComputer{}, os.Stdout)
	if errorValue != nil {
		return errorValue
	}
	printBackup(os.Stdout, result)
	return nil
}

func runSnapshot(arguments []string) error {
	flags := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	keep := flags.Int("keep", 0, "keep only this many of the newest snapshots for the company's boxes")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("snapshot takes no arguments, only --keep")
	}
	if errorValue := companyhost.RequireAdministrator(); errorValue != nil {
		return errorValue
	}
	now := time.Now()
	shipment, isShipped, errorValue := thisBox().Ship(context.Background(), *keep, now, func(snapshot *offsite.Snapshot) (companyhost.Snapshot, error) {
		return companyhost.SnapshotTo(snapshot, now, thisComputer{}, os.Stdout)
	})
	if errorValue != nil {
		return errorValue
	}
	printShipment(os.Stdout, shipment, isShipped)
	return nil
}

func thisBox() offsite.ThisBox {
	return offsite.ThisBox{
		BoxStatePath:    blueclaw.CompanyHostBoxStatePath,
		ConnectionPath:  companyhost.CurrentConnectionPath(),
		AccessTokenPath: blueclaw.CompanyHostAgentKeyPath,
	}
}

func printShipment(output io.Writer, shipment offsite.Shipment, isShipped bool) {
	if !isShipped {
		fmt.Fprintln(output, "This computer is not a box serving its company with the company key, so it keeps no snapshot.")
		return
	}
	fmt.Fprintf(output, "Kept %s for the company's boxes, sealed to the company key; %d chunks were new.\n", shipment.Manifest.Name, shipment.UploadedChunks)
	for _, pruned := range shipment.Pruned {
		fmt.Fprintf(output, "Removed the older snapshot %s.\n", pruned)
	}
}

func printBackup(output io.Writer, result companyhost.BackupResult) {
	fmt.Fprintf(output, "\nBacked up company %s to %s (%s).\n", result.Manifest.CompanyID, result.ArchivePath, companyhost.DescribeSize(result.Manifest.TotalSize()))
	for _, member := range result.Manifest.Members {
		fmt.Fprintf(output, "  %-24s %s\n", member.Name, companyhost.DescribeSize(member.Size))
	}
	fmt.Fprintf(output, "  %d files under %v\n", result.Files.Files, result.Manifest.FileRoots)
	for _, warning := range result.Files.Warnings {
		fmt.Fprintf(output, "  note: %s\n", warning)
	}
	for _, pruned := range result.Pruned {
		fmt.Fprintf(output, "Removed the older backup %s.\n", pruned)
	}
	fmt.Fprintln(output, "The archive here holds the company's keys and its people's data, unencrypted.")
}

func runRestore(arguments []string) error {
	flags := flag.NewFlagSet("restore", flag.ContinueOnError)
	isReplacing := flags.Bool("replace", false, "replace the company this computer already runs, after backing up what it holds now")
	isTakingOver := flags.Bool("taking-over", false, "this box was made the company's service box: restore what the last one kept, and stay this box")
	archivePaths := []string{}
	remaining := arguments
	for {
		if errorValue := flags.Parse(remaining); errorValue != nil {
			return errorValue
		}
		if flags.NArg() == 0 {
			break
		}
		archivePaths = append(archivePaths, flags.Arg(0))
		remaining = flags.Args()[1:]
	}
	if len(archivePaths) != 1 {
		return fmt.Errorf("name the one backup archive to restore")
	}
	if errorValue := companyhost.RequireAdministrator(); errorValue != nil {
		return errorValue
	}
	request := companyhost.RestoreRequest{
		ArchivePath: archivePaths[0],
		Replace:     *isReplacing,
		Now:         time.Now(),
	}
	if *isTakingOver {
		request.Replace, request.KeepsThisBox, request.BeforeStarting = true, true, markThisBoxInstalled
	}
	result, errorValue := companyhost.Restore(request, thisComputer{}, os.Stdout)
	if errorValue != nil {
		return errorValue
	}
	printRestore(os.Stdout, result)
	return nil
}

func printRestore(output io.Writer, result companyhost.RestoreResult) {
	fmt.Fprintf(output, "\nRestored %s from the backup made %s.\n", result.Connection.Company.Name, result.Manifest.CreatedAt.Local().Format("2006-01-02 15:04"))
	fmt.Fprintf(output, "  %d files, %s, under %v\n", result.Files.Files, companyhost.DescribeSize(result.Files.Bytes), result.Manifest.FileRoots)
	printUnresolved(output, result.Unresolved)
	if result.SafetyBackup != "" {
		fmt.Fprintf(output, "What this computer held before is in %s; restore it with --replace to undo this.\n", result.SafetyBackup)
	}
	fmt.Fprintf(output, "%s runs the server again. Open %s/settings/setup and choose Check connection.\n", result.Supervisor, result.Connection.AppURL)
}

func printUnresolved(output io.Writer, unresolved []hostbackup.Ownership) {
	if len(unresolved) == 0 {
		return
	}
	accounts := map[string]int{}
	for _, ownership := range unresolved {
		accounts[ownership.UserName+":"+ownership.GroupName]++
	}
	fmt.Fprintf(output, "  %d entries belonged to accounts this computer does not have, and root owns them now:\n", len(unresolved))
	for account, count := range accounts {
		fmt.Fprintf(output, "    %s (%d)\n", account, count)
	}
}
