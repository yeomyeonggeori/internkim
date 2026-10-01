package hostbackup

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
)

type Ownership struct {
	Path      string
	UserName  string
	GroupName string
	Mode      fs.FileMode
	IsLink    bool
}

type ExtractionReport struct {
	Files      int
	Bytes      int64
	Unresolved []Ownership
}

type extraction struct {
	destinations map[string]string
	verified     map[string]bool
	accounts     *Accounts
	report       ExtractionReport
}

func ExtractFiles(input io.Reader, destinations map[string]string) (ExtractionReport, error) {
	extracting := &extraction{destinations: destinations, verified: map[string]bool{}, accounts: NewAccounts()}
	reader := tar.NewReader(input)
	for {
		header, errorValue := reader.Next()
		if errors.Is(errorValue, io.EOF) {
			return extracting.report, nil
		}
		if errorValue != nil {
			return ExtractionReport{}, fmt.Errorf("reading %s: %w", FilesMemberName, errorValue)
		}
		if errorValue := extracting.extract(header, reader); errorValue != nil {
			return ExtractionReport{}, fmt.Errorf("restoring %s: %w", header.Name, errorValue)
		}
	}
}

func (extracting *extraction) destinationOf(archivedName string) (string, error) {
	role, relative, errorValue := splitArchivedName(archivedName)
	if errorValue != nil {
		return "", errorValue
	}
	root, isKnown := extracting.destinations[role]
	if !isKnown {
		return "", fmt.Errorf("it belongs to %q, which is not a root this backup lists", role)
	}
	if errorValue := extracting.ensureRoot(root); errorValue != nil {
		return "", errorValue
	}
	if relative == "." {
		return root, nil
	}
	destination := filepath.Join(root, filepath.FromSlash(relative))
	if errorValue := extracting.refuseLinkedAncestors(root, filepath.Dir(destination)); errorValue != nil {
		return "", errorValue
	}
	return destination, nil
}

func (extracting *extraction) ensureRoot(root string) error {
	if extracting.verified[root] {
		return nil
	}
	if errorValue := os.MkdirAll(root, 0o700); errorValue != nil {
		return errorValue
	}
	extracting.verified[root] = true
	return nil
}

func (extracting *extraction) refuseLinkedAncestors(root string, directory string) error {
	if directory == root || extracting.verified[directory] {
		return nil
	}
	if errorValue := extracting.refuseLinkedAncestors(root, filepath.Dir(directory)); errorValue != nil {
		return errorValue
	}
	information, errorValue := os.Lstat(directory)
	if errors.Is(errorValue, fs.ErrNotExist) {
		return os.Mkdir(directory, 0o700)
	}
	if errorValue != nil {
		return errorValue
	}
	if !information.IsDir() {
		return fmt.Errorf("%s is not a directory, and something in the backup would be written through it", directory)
	}
	extracting.verified[directory] = true
	return nil
}

func (extracting *extraction) extract(header *tar.Header, reader io.Reader) error {
	destination, errorValue := extracting.destinationOf(header.Name)
	if errorValue != nil {
		return errorValue
	}
	switch header.Typeflag {
	case tar.TypeDir:
		return extracting.extractDirectory(destination, header)
	case tar.TypeSymlink:
		return extracting.extractSymlink(destination, header)
	case tar.TypeLink:
		return extracting.extractHardLink(destination, header)
	case tar.TypeReg:
		return extracting.extractFile(destination, header, reader)
	}
	return fmt.Errorf("it is a tar entry of type %q, which a backup never holds", header.Typeflag)
}

func (extracting *extraction) extractDirectory(destination string, header *tar.Header) error {
	if errorValue := os.Mkdir(destination, 0o700); errorValue != nil && !errors.Is(errorValue, fs.ErrExist) {
		return errorValue
	}
	information, errorValue := os.Lstat(destination)
	if errorValue != nil {
		return errorValue
	}
	if !information.IsDir() {
		return fmt.Errorf("%s already exists and is not a directory", destination)
	}
	extracting.verified[destination] = true
	return extracting.own(destination, header, false)
}

func (extracting *extraction) extractSymlink(destination string, header *tar.Header) error {
	if errorValue := removeExisting(destination); errorValue != nil {
		return errorValue
	}
	if errorValue := os.Symlink(header.Linkname, destination); errorValue != nil {
		return errorValue
	}
	return extracting.own(destination, header, true)
}

func (extracting *extraction) extractHardLink(destination string, header *tar.Header) error {
	target, errorValue := extracting.destinationOf(header.Linkname)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := removeExisting(destination); errorValue != nil {
		return errorValue
	}
	return os.Link(target, destination)
}

func (extracting *extraction) extractFile(destination string, header *tar.Header, reader io.Reader) error {
	if errorValue := removeExisting(destination); errorValue != nil {
		return errorValue
	}
	file, errorValue := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
	if errorValue != nil {
		return errorValue
	}
	copied, copyError := io.Copy(file, reader)
	closeError := file.Close()
	if copyError != nil {
		return copyError
	}
	if closeError != nil {
		return closeError
	}
	extracting.report.Files++
	extracting.report.Bytes += copied
	if errorValue := extracting.own(destination, header, false); errorValue != nil {
		return errorValue
	}
	return os.Chtimes(destination, header.ModTime, header.ModTime)
}

func removeExisting(destination string) error {
	information, errorValue := os.Lstat(destination)
	if errors.Is(errorValue, fs.ErrNotExist) {
		return nil
	}
	if errorValue != nil {
		return errorValue
	}
	if information.IsDir() {
		return fmt.Errorf("%s already exists as a directory", destination)
	}
	return os.Remove(destination)
}

func (extracting *extraction) own(destination string, header *tar.Header, isLink bool) error {
	ownership := Ownership{
		Path:      destination,
		UserName:  header.Uname,
		GroupName: header.Gname,
		Mode:      header.FileInfo().Mode(),
		IsLink:    isLink,
	}
	isResolved, errorValue := ApplyOwnership(ownership, extracting.accounts)
	if errorValue != nil {
		return errorValue
	}
	if !isResolved {
		extracting.report.Unresolved = append(extracting.report.Unresolved, ownership)
	}
	return nil
}

func ApplyOwnership(ownership Ownership, accounts *Accounts) (bool, error) {
	userID, isUserKnown := accounts.userID(ownership.UserName)
	groupID, isGroupKnown := accounts.groupID(ownership.GroupName)
	if !isUserKnown {
		userID = unchangedOwner
	}
	if !isGroupKnown {
		groupID = unchangedOwner
	}
	if errorValue := os.Lchown(ownership.Path, userID, groupID); errorValue != nil {
		return false, errorValue
	}
	if ownership.IsLink {
		return isUserKnown && isGroupKnown, nil
	}
	if errorValue := os.Chmod(ownership.Path, ownership.Mode.Perm()|ownership.Mode&(fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky)); errorValue != nil {
		return false, errorValue
	}
	return isUserKnown && isGroupKnown, nil
}

const unchangedOwner = -1

type Accounts struct {
	users  map[string]int
	groups map[string]int
}

func NewAccounts() *Accounts {
	return &Accounts{users: map[string]int{}, groups: map[string]int{}}
}

func (accounts *Accounts) userID(name string) (int, bool) {
	if identifier, isCached := accounts.users[name]; isCached {
		return identifier, true
	}
	account, errorValue := user.Lookup(name)
	if errorValue != nil {
		return 0, false
	}
	identifier, errorValue := strconv.Atoi(account.Uid)
	if errorValue != nil {
		return 0, false
	}
	accounts.users[name] = identifier
	return identifier, true
}

func (accounts *Accounts) groupID(name string) (int, bool) {
	if identifier, isCached := accounts.groups[name]; isCached {
		return identifier, true
	}
	group, errorValue := user.LookupGroup(name)
	if errorValue != nil {
		return 0, false
	}
	identifier, errorValue := strconv.Atoi(group.Gid)
	if errorValue != nil {
		return 0, false
	}
	accounts.groups[name] = identifier
	return identifier, true
}
