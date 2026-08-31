package admind

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// The circle everyone belongs to was called staff until internkim#507. A device
// provisioned before that holds its files at the old path, owned by a group of
// the old name, and both have to arrive at the new one without the files
// changing hands.
//
// This runs where the rest of the workspace repairs run rather than in the
// guest's init, because the init lives in a four-gigabyte rootfs whose install
// stops the agent for the length of the transfer.
const (
	circleDirectoryName    = "circles"
	memberCircleName       = "member"
	formerMemberCircleName = "staff"
	formerMemberGroupName  = "bc_circle_staff"
)

func (service *Service) renameTheFormerStaffCircle() {
	workspace := service.Configuration.BlueclawWorkspacePath
	if workspace == "" {
		return
	}
	former := filepath.Join(workspace, circleDirectoryName, formerMemberCircleName)
	if _, errorValue := os.Stat(former); errorValue != nil {
		return
	}

	renameTheFormerMemberGroup()

	current := filepath.Join(workspace, circleDirectoryName, memberCircleName)
	if _, errorValue := os.Stat(current); os.IsNotExist(errorValue) {
		if errorValue := os.Rename(former, current); errorValue != nil {
			log.Printf("member circle migration: %v", errorValue)
			return
		}
		log.Printf("member circle migration: %s is now %s", former, current)
		return
	}

	if errorValue := moveCircleContents(former, current); errorValue != nil {
		log.Printf("member circle migration: %v", errorValue)
		return
	}
	if errorValue := os.Remove(former); errorValue != nil {
		log.Printf("member circle migration: the old circle is empty and still there: %v", errorValue)
		return
	}
	log.Printf("member circle migration: what %s held is now under %s", former, current)
}

// A new group is a new GID, and every file the old one owns stops being
// readable by the people who own it. So the group is renamed, never remade.
func renameTheFormerMemberGroup() {
	if _, errorValue := exec.LookPath("groupmod"); errorValue != nil {
		return
	}
	if exec.Command("getent", "group", formerMemberGroupName).Run() != nil {
		return
	}
	if exec.Command("getent", "group", memberCircleGroupName).Run() == nil {
		log.Printf("member circle migration: %s already exists, so %s is left alone rather than merged", memberCircleGroupName, formerMemberGroupName)
		return
	}
	if output, errorValue := exec.Command("groupmod", "-n", memberCircleGroupName, formerMemberGroupName).CombinedOutput(); errorValue != nil {
		log.Printf("member circle migration: renaming %s failed, so the files keep their group: %v: %s", formerMemberGroupName, errorValue, output)
		return
	}
	log.Printf("member circle migration: the group %s is now %s, keeping its id", formerMemberGroupName, memberCircleGroupName)
}

func moveCircleContents(former string, current string) error {
	entries, errorValue := os.ReadDir(former)
	if errorValue != nil {
		return errorValue
	}
	for _, entry := range entries {
		from := filepath.Join(former, entry.Name())
		to := filepath.Join(current, entry.Name())
		if _, errorValue := os.Stat(to); errorValue == nil {
			log.Printf("member circle migration: %s is already there, leaving %s", to, from)
			continue
		}
		if errorValue := os.Rename(from, to); errorValue != nil {
			return errorValue
		}
	}
	return nil
}
