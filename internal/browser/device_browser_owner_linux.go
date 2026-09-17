package browser

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"strconv"
	"syscall"
)

const deviceBrowserFallbackUserName = "nobody"

type deviceBrowserOwner struct {
	userID  uint32
	groupID uint32
}

func deviceBrowserOwnerOf(userName string) (*deviceBrowserOwner, error) {
	if os.Geteuid() != 0 {
		return nil, nil
	}
	account, errorValue := user.Lookup(userName)
	if errorValue != nil {
		log.Printf("device browser user %q does not exist, so the browser runs as %q: %v", userName, deviceBrowserFallbackUserName, errorValue)
		account, errorValue = user.Lookup(deviceBrowserFallbackUserName)
	}
	if errorValue != nil {
		return nil, fmt.Errorf("no user to run the device browser as: %w", errorValue)
	}
	userID, errorValue := strconv.ParseUint(account.Uid, 10, 32)
	if errorValue != nil {
		return nil, fmt.Errorf("user %s has a non-numeric id %q", account.Username, account.Uid)
	}
	groupID, errorValue := strconv.ParseUint(account.Gid, 10, 32)
	if errorValue != nil {
		return nil, fmt.Errorf("user %s has a non-numeric group id %q", account.Username, account.Gid)
	}
	return &deviceBrowserOwner{userID: uint32(userID), groupID: uint32(groupID)}, nil
}

func (owner *deviceBrowserOwner) own(path string) error {
	if owner == nil {
		return nil
	}
	if errorValue := os.Chown(path, int(owner.userID), int(owner.groupID)); errorValue != nil {
		return fmt.Errorf("the device browser path %s could not be handed to its user: %w", path, errorValue)
	}
	return nil
}

func deviceBrowserProcessAttributes(owner *deviceBrowserOwner) *syscall.SysProcAttr {
	attributes := &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	if owner != nil {
		attributes.Credential = &syscall.Credential{Uid: owner.userID, Gid: owner.groupID}
	}
	return attributes
}
