package deviceexport

import (
	"archive/tar"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
)

const (
	privateDirectoryMode = 0o700
	privateFileMode      = 0o600
	rootName             = "root"
)

type filesWriter struct {
	tarWriter *tar.Writer
	report    *Report
	modified  time.Time
}

func writeFiles(output io.Writer, exported string, connection companyhost.Connection, device deviceState, guestNames accountNames, hostNames accountNames, report *Report) error {
	writer := &filesWriter{tarWriter: tar.NewWriter(output), report: report, modified: time.Now().Truncate(time.Second)}
	steps := []func() error{
		func() error { return writer.writeCompany(connection, device) },
		func() error { return writer.writeDeviceState(filepath.Join(exported, hostStateArchiveName), hostNames) },
		func() error { return writer.writeMedia(filepath.Join(exported, mediaDirectoryName)) },
		func() error { return writer.writeWorkspace(filepath.Join(exported, workspaceArchiveName), guestNames) },
	}
	for _, step := range steps {
		if errorValue := step(); errorValue != nil {
			return errorValue
		}
	}
	return writer.tarWriter.Close()
}

// writeCompany is the one part no device file becomes as it is: the connection
// the host package keeps, and the keys it keeps beside it, under the names the
// package gives them.
func (writer *filesWriter) writeCompany(connection companyhost.Connection, device deviceState) error {
	companyDirectory := path.Join(stateRole, "companies", connection.Company.ID)
	for _, directory := range []string{stateRole, path.Join(stateRole, "companies"), companyDirectory, path.Join(companyDirectory, "secrets")} {
		if errorValue := writer.writeDirectory(directory, rootName, rootName, privateDirectoryMode); errorValue != nil {
			return errorValue
		}
	}
	document, errorValue := json.MarshalIndent(connection, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := writer.writeBytes(path.Join(companyDirectory, "connection.json"), append(document, '\n')); errorValue != nil {
		return errorValue
	}
	for name, contents := range device.secrets {
		if errorValue := writer.writeBytes(path.Join(companyDirectory, "secrets", name), contents); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

// writeDeviceState routes the device's own files to where the host package
// keeps the same thing. The admin gateway's directory keeps its path; chatd's
// state and the messenger's account links move under the host's state root;
// the relay's queues move to the relay's root.
func (writer *filesWriter) writeDeviceState(archivePath string, hostNames accountNames) error {
	return eachEntry(archivePath, func(header *tar.Header, reader io.Reader) error {
		name := strings.TrimSuffix(strings.TrimPrefix(header.Name, "./"), "/")
		destinations := deviceDestinations(name)
		if len(destinations) == 0 {
			writer.leaveOut(name)
			return nil
		}
		if header.Typeflag == tar.TypeReg && len(destinations) > 1 {
			document, errorValue := io.ReadAll(reader)
			if errorValue != nil {
				return errorValue
			}
			for _, destination := range destinations {
				if errorValue := writer.writeRenamed(header, destination, hostNames, strings.NewReader(string(document))); errorValue != nil {
					return errorValue
				}
			}
			return nil
		}
		return writer.writeRenamed(header, destinations[0], hostNames, reader)
	})
}

func deviceDestinations(name string) []string {
	destinations := []string{}
	if relative, isAdministration := strings.CutPrefix(name, deviceAdministrationPrefix); isAdministration && isAdministrationCarried(relative) {
		destinations = append(destinations, path.Join(administrationRole, relative))
	}
	if relative, isChatd := strings.CutPrefix(name, deviceChatdPrefix); isChatd {
		destinations = append(destinations, path.Join(stateRole, "chatd", relative))
	}
	if name == strings.TrimSuffix(deviceChatdPrefix, "/") {
		destinations = append(destinations, path.Join(stateRole, "chatd"))
	}
	if name == deviceAccountLinksPath {
		destinations = append(destinations, path.Join(stateRole, "buzz-account-links.json"))
	}
	if relative, isRelay := strings.CutPrefix(name, deviceRelayPrefix); isRelay {
		destinations = append(destinations, path.Join(relayRole, relative))
	}
	return destinations
}

func isAdministrationCarried(relative string) bool {
	if relative == "" || isUnder(relative, administrationLeftOut) {
		return false
	}
	if relative == "secrets" {
		return true
	}
	if !strings.HasPrefix(relative, "secrets/") {
		return true
	}
	return isUnder(relative, administrationSecretsKept)
}

func (writer *filesWriter) leaveOut(name string) {
	segments := strings.Split(name, "/")
	topLevel := strings.Join(segments[:min(3, len(segments))], "/")
	for _, already := range writer.report.LeftOut {
		if already == topLevel {
			return
		}
	}
	writer.report.LeftOut = append(writer.report.LeftOut, topLevel)
}

// writeMedia carries the media store's mirror into the bucket directories the
// host's object store serves. The device's MinIO kept its own format, so the
// export mirrored it as plain files through S3.
func (writer *filesWriter) writeMedia(mediaPath string) error {
	if _, errorValue := os.Stat(mediaPath); os.IsNotExist(errorValue) {
		return nil
	}
	mediaRoot := path.Join(stateRole, "media")
	if errorValue := writer.writeDirectory(mediaRoot, rootName, rootName, 0o755); errorValue != nil {
		return errorValue
	}
	return filepath.WalkDir(mediaPath, func(filePath string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relative, errorValue := filepath.Rel(mediaPath, filePath)
		if errorValue != nil || relative == "." {
			return errorValue
		}
		archived := path.Join(mediaRoot, filepath.ToSlash(relative))
		if entry.IsDir() {
			return writer.writeDirectory(archived, rootName, rootName, 0o755)
		}
		return writer.writeFile(archived, filePath)
	})
}

// writeWorkspace renames every entry under the workspace root and names its
// owner: the guest's own accounts from its passwd and group, and every person,
// circle and the shared group from the helper's identity map.
func (writer *filesWriter) writeWorkspace(archivePath string, guestNames accountNames) error {
	return eachEntry(archivePath, func(header *tar.Header, reader io.Reader) error {
		relative := strings.TrimSuffix(strings.TrimPrefix(header.Name, "./"), "/")
		if relative == "." {
			relative = ""
		}
		if relative != "" && isUnder(relative, workspaceLeftOut) {
			return nil
		}
		destination := workspaceRole
		if relative != "" {
			destination = path.Join(workspaceRole, relative)
		}
		if userName := guestNames.user(header.Uid); userName != "" {
			writer.report.WorkspaceOwners[userName]++
		}
		return writer.writeRenamed(header, destination, guestNames, reader)
	})
}

func (writer *filesWriter) writeRenamed(header *tar.Header, destination string, names accountNames, reader io.Reader) error {
	renamed := &tar.Header{
		Typeflag: header.Typeflag,
		Name:     destination,
		Linkname: header.Linkname,
		Size:     header.Size,
		Mode:     header.Mode,
		ModTime:  header.ModTime,
		Uid:      header.Uid,
		Gid:      header.Gid,
		Uname:    names.user(header.Uid),
		Gname:    names.group(header.Gid),
		Format:   tar.FormatPAX,
	}
	if renamed.Uname == "" || renamed.Gname == "" {
		writer.report.UnnamedOwners++
	}
	switch header.Typeflag {
	case tar.TypeDir:
		renamed.Name += "/"
		renamed.Size = 0
	case tar.TypeLink:
		renamed.Linkname = linkedName(destination, header)
		renamed.Size = 0
	case tar.TypeSymlink:
		renamed.Size = 0
	case tar.TypeReg:
	default:
		writer.leaveOut(destination)
		return nil
	}
	if errorValue := writer.tarWriter.WriteHeader(renamed); errorValue != nil {
		return errorValue
	}
	writer.report.Entries++
	if header.Typeflag != tar.TypeReg {
		return nil
	}
	_, errorValue := io.Copy(writer.tarWriter, reader)
	return errorValue
}

// A hard link names its target the way its own archive did, so the target
// moves under the same root the link does.
func linkedName(destination string, header *tar.Header) string {
	role := strings.SplitN(destination, "/", 2)[0]
	target := strings.TrimPrefix(strings.TrimPrefix(header.Linkname, "./"), "/")
	if role == workspaceRole {
		return path.Join(workspaceRole, target)
	}
	for _, candidate := range deviceDestinations(target) {
		if strings.SplitN(candidate, "/", 2)[0] == role {
			return candidate
		}
	}
	return path.Join(role, target)
}

func (writer *filesWriter) writeDirectory(name string, userName string, groupName string, mode int64) error {
	writer.report.Entries++
	return writer.tarWriter.WriteHeader(&tar.Header{
		Typeflag: tar.TypeDir, Name: name + "/", Mode: mode, ModTime: writer.modified,
		Uname: userName, Gname: groupName, Format: tar.FormatPAX,
	})
}

func (writer *filesWriter) writeBytes(name string, contents []byte) error {
	writer.report.Entries++
	if errorValue := writer.tarWriter.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg, Name: name, Size: int64(len(contents)), Mode: privateFileMode, ModTime: writer.modified,
		Uname: rootName, Gname: rootName, Format: tar.FormatPAX,
	}); errorValue != nil {
		return errorValue
	}
	_, errorValue := writer.tarWriter.Write(contents)
	return errorValue
}

func (writer *filesWriter) writeFile(name string, filePath string) error {
	information, errorValue := os.Stat(filePath)
	if errorValue != nil {
		return errorValue
	}
	file, errorValue := os.Open(filePath)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	writer.report.Entries++
	if errorValue := writer.tarWriter.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg, Name: name, Size: information.Size(), Mode: 0o644, ModTime: information.ModTime(),
		Uname: rootName, Gname: rootName, Format: tar.FormatPAX,
	}); errorValue != nil {
		return errorValue
	}
	_, errorValue = io.Copy(writer.tarWriter, file)
	return errorValue
}
