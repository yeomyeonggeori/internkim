package filestream

import "github.com/yeomyeonggeori/internkim/internal/hostbackup"

type Change struct {
	Root       hostbackup.FileRoot
	Relative   string
	IsOverflow bool
}

func (change Change) archivedName() string {
	return change.Root.Role + "/" + change.Relative
}
