package companyhost

import (
	"path/filepath"
	"slices"

	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const BarrierRole = stateRole

func JournaledFileRoots(layout blueclaw.CompanyHostLayout, barrierDirectoryName string) []hostbackup.FileRoot {
	roots := hostFileRoots(layout)
	for index, root := range roots {
		if root.Role == stateRole {
			roots[index].Excluded = append(slices.Clone(root.Excluded), filepath.Base(blueclaw.CompanyHostBoxStatePath), barrierDirectoryName)
		}
	}
	return roots
}
