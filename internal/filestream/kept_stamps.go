package filestream

import (
	"io/fs"
	"time"
)

const settledAfter = time.Second

type keptStamp struct {
	changedAtNanoseconds int64
	size                 int64
	mode                 fs.FileMode
}

type keptStamps map[string]keptStamp

func stampOf(information fs.FileInfo) keptStamp {
	return keptStamp{
		changedAtNanoseconds: lastChangeOf(information).UnixNano(),
		size:                 information.Size(),
		mode:                 information.Mode(),
	}
}

func (stamps keptStamps) holds(name string, stamp keptStamp) bool {
	kept, isKept := stamps[name]
	return isKept && kept == stamp
}

func (stamps keptStamps) remember(name string, stamp keptStamp, takenAt time.Time) {
	if takenAt.Sub(time.Unix(0, stamp.changedAtNanoseconds)) <= settledAfter {
		delete(stamps, name)
		return
	}
	stamps[name] = stamp
}
