package companyhost

import (
	"fmt"
	"io"

	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
)

const dumpMagic = "PGDMP"

type dumpFormat struct {
	Major int
	Minor int
}

func (format dumpFormat) String() string {
	return fmt.Sprintf("%d.%d", format.Major, format.Minor)
}

func (format dumpFormat) isNewerThan(other dumpFormat) bool {
	if format.Major != other.Major {
		return format.Major > other.Major
	}
	return format.Minor > other.Minor
}

type formatIntroduction struct {
	PostgreSQLMajor int
	Format          dumpFormat
}

var dumpFormatsBySuccessiveMajor = []formatIntroduction{
	{PostgreSQLMajor: 14, Format: dumpFormat{Major: 1, Minor: 14}},
	{PostgreSQLMajor: 16, Format: dumpFormat{Major: 1, Minor: 15}},
	{PostgreSQLMajor: 17, Format: dumpFormat{Major: 1, Minor: 16}},
}

func newestFormatReadBy(postgreSQLMajor int) dumpFormat {
	newest := dumpFormatsBySuccessiveMajor[0].Format
	for _, introduction := range dumpFormatsBySuccessiveMajor {
		if introduction.PostgreSQLMajor <= postgreSQLMajor {
			newest = introduction.Format
		}
	}
	return newest
}

func oldestMajorReading(format dumpFormat) int {
	for _, introduction := range dumpFormatsBySuccessiveMajor {
		if !format.isNewerThan(introduction.Format) {
			return introduction.PostgreSQLMajor
		}
	}
	return dumpFormatsBySuccessiveMajor[len(dumpFormatsBySuccessiveMajor)-1].PostgreSQLMajor
}

func formatOfDump(input io.Reader) (dumpFormat, bool) {
	header := make([]byte, len(dumpMagic)+2)
	if _, errorValue := io.ReadFull(input, header); errorValue != nil {
		return dumpFormat{}, false
	}
	if string(header[:len(dumpMagic)]) != dumpMagic {
		return dumpFormat{}, false
	}
	return dumpFormat{Major: int(header[len(dumpMagic)]), Minor: int(header[len(dumpMagic)+1])}, true
}

func refuseADumpTheHostCannotRead(platform backupPlatform, archive hostbackup.Archive, machine Machine) error {
	hostMajor, errorValue := platform.DatabaseMajor(machine)
	if errorValue != nil {
		return errorValue
	}
	readable := newestFormatReadBy(hostMajor)
	for _, member := range archive.Manifest.Members {
		if _, isDatabase := hostbackup.DatabaseNamedBy(member.Name); !isDatabase {
			continue
		}
		if errorValue := refuseAMemberTheHostCannotRead(archive, member.Name, hostMajor, readable); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func refuseAMemberTheHostCannotRead(archive hostbackup.Archive, name string, hostMajor int, readable dumpFormat) error {
	return archive.ReadMember(name, func(input io.Reader) error {
		format, isDump := formatOfDump(input)
		if !isDump || !format.isNewerThan(readable) {
			return nil
		}
		return fmt.Errorf(
			"%s is a dump in archive format %s, and the pg_restore of this computer's PostgreSQL %d reads up to %s. Install PostgreSQL %d or newer here first",
			name, format, hostMajor, readable, oldestMajorReading(format))
	})
}
