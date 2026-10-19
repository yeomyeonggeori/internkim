package offsite

import (
	"crypto/sha256"
	"encoding/binary"
)

const (
	smallestChunkBytes = 512 * 1024
	typicalChunkBytes  = 2 * 1024 * 1024
	largestChunkBytes  = 8 * 1024 * 1024
	strictBoundaryMask = typicalChunkBytes*4 - 1
	looseBoundaryMask  = typicalChunkBytes/4 - 1
)

var gearTable = gearTableOf("internkim content-defined chunking gear table, version 1")

func gearTableOf(seed string) [256]uint64 {
	var table [256]uint64
	for index := range table {
		digest := sha256.Sum256(append([]byte(seed), byte(index)))
		table[index] = binary.LittleEndian.Uint64(digest[:8])
	}
	return table
}

type chunker struct {
	emit        func(chunk []byte) error
	pending     []byte
	scanned     int
	fingerprint uint64
}

func newChunker(emit func(chunk []byte) error) *chunker {
	return &chunker{emit: emit, pending: make([]byte, 0, largestChunkBytes), scanned: smallestChunkBytes}
}

func (splitter *chunker) Write(bytes []byte) (int, error) {
	written := len(bytes)
	for len(bytes) > 0 {
		taken := min(largestChunkBytes-len(splitter.pending), len(bytes))
		splitter.pending = append(splitter.pending, bytes[:taken]...)
		bytes = bytes[taken:]
		if errorValue := splitter.emitAtBoundaries(); errorValue != nil {
			return 0, errorValue
		}
	}
	return written, nil
}

func (splitter *chunker) Close() error {
	if len(splitter.pending) == 0 {
		return nil
	}
	return splitter.emitUpTo(len(splitter.pending))
}

func (splitter *chunker) emitAtBoundaries() error {
	for splitter.scanned < len(splitter.pending) {
		splitter.fingerprint = splitter.fingerprint<<1 + gearTable[splitter.pending[splitter.scanned]]
		splitter.scanned++
		if splitter.isAtBoundary() {
			if errorValue := splitter.emitUpTo(splitter.scanned); errorValue != nil {
				return errorValue
			}
		}
	}
	return nil
}

func (splitter *chunker) isAtBoundary() bool {
	if splitter.scanned >= largestChunkBytes {
		return true
	}
	if splitter.scanned < typicalChunkBytes {
		return splitter.fingerprint&strictBoundaryMask == 0
	}
	return splitter.fingerprint&looseBoundaryMask == 0
}

func (splitter *chunker) emitUpTo(boundary int) error {
	if errorValue := splitter.emit(splitter.pending[:boundary]); errorValue != nil {
		return errorValue
	}
	splitter.pending = append(splitter.pending[:0], splitter.pending[boundary:]...)
	splitter.scanned = smallestChunkBytes
	splitter.fingerprint = 0
	return nil
}
