package deviceexport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// accountNames turns the numbers a tar recorded into the names a backup
// records. A number nothing names stays empty, and the restore leaves that
// file to root and lists it.
type accountNames struct {
	users  map[int]string
	groups map[int]string
}

func newAccountNames() accountNames {
	return accountNames{users: map[int]string{}, groups: map[int]string{}}
}

func (names accountNames) user(identifier int) string {
	return names.users[identifier]
}

func (names accountNames) group(identifier int) string {
	return names.groups[identifier]
}

// readAccountFile reads /etc/passwd or /etc/group lines, or the shorter
// name:id lines `cut -d: -f1,3` prints. The guest's init gives the agent's
// account a number its root filesystem already gave systemd-network, and every
// file the agent wrote is the agent's, so that name wins a shared number.
func readAccountFile(path string, into map[int]string) error {
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	return readAccountLines(file, into)
}

func readAccountLines(input io.Reader, into map[int]string) error {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		fields := strings.Split(strings.TrimSpace(scanner.Text()), ":")
		if len(fields) < 2 || fields[0] == "" {
			continue
		}
		identifierField := fields[1]
		if len(fields) >= 3 {
			identifierField = fields[2]
		}
		identifier, errorValue := strconv.Atoi(identifierField)
		if errorValue != nil {
			continue
		}
		if _, isNamed := into[identifier]; !isNamed || fields[0] == blueclaw.BlueclawUser {
			into[identifier] = fields[0]
		}
	}
	return scanner.Err()
}

// addIdentityMap names the bc_person_*, bc_circle_* and bc_shared identities
// the guest's POSIX helper allocated. One number belongs to one name, and a
// person's private group carries the same number as the person.
func (names accountNames) addIdentityMap(document []byte) error {
	allocations := map[string]int{}
	if errorValue := json.Unmarshal(document, &allocations); errorValue != nil {
		return fmt.Errorf("the guest's identity map is not the name-to-number table the helper writes: %w", errorValue)
	}
	for name, identifier := range allocations {
		names.users[identifier] = name
		names.groups[identifier] = name
	}
	return nil
}
