package rpmrepository

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
)

const (
	leadLength          = 96
	headerMagicLength   = 8
	indexEntryLength    = 16
	signatureRegionTag  = 62
	immutableRegionTag  = 63
	signatureAlignment  = 8
	typeCharacter       = 1
	typeInteger8        = 2
	typeInteger16       = 3
	typeInteger32       = 4
	typeInteger64       = 5
	typeString          = 6
	typeBinary          = 7
	typeStringArray     = 8
	typeInternationalis = 9
)

var leadMagic = []byte{0xed, 0xab, 0xee, 0xdb}
var headerMagic = []byte{0x8e, 0xad, 0xe8, 0x01, 0, 0, 0, 0}

type headerEntry struct {
	tag   int
	kind  int
	count int
	data  []byte
}

type header struct {
	entries map[int]headerEntry
	length  int
}

func (parsed header) strings(tag int) []string {
	entry, present := parsed.entries[tag]
	if !present {
		return nil
	}
	switch entry.kind {
	case typeString, typeStringArray, typeInternationalis:
		parts := bytes.Split(entry.data, []byte{0})
		values := make([]string, 0, entry.count)
		for index := 0; index < entry.count && index < len(parts); index++ {
			values = append(values, string(parts[index]))
		}
		return values
	}
	return nil
}

func (parsed header) text(tag int) string {
	values := parsed.strings(tag)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (parsed header) integers(tag int) []uint64 {
	entry, present := parsed.entries[tag]
	if !present {
		return nil
	}
	width := map[int]int{typeInteger8: 1, typeCharacter: 1, typeInteger16: 2, typeInteger32: 4, typeInteger64: 8}[entry.kind]
	if width == 0 {
		return nil
	}
	values := make([]uint64, 0, entry.count)
	for index := 0; index < entry.count; index++ {
		chunk := entry.data[index*width : (index+1)*width]
		var value uint64
		for _, octet := range chunk {
			value = value<<8 | uint64(octet)
		}
		values = append(values, value)
	}
	return values
}

func (parsed header) integer(tag int) uint64 {
	values := parsed.integers(tag)
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func readHeader(archive []byte, offset int) (header, error) {
	if offset+headerMagicLength+8 > len(archive) || !bytes.Equal(archive[offset:offset+4], headerMagic[:4]) {
		return header{}, fmt.Errorf("no rpm header at byte %d", offset)
	}
	entryCount := int(binary.BigEndian.Uint32(archive[offset+8:]))
	storeLength := int(binary.BigEndian.Uint32(archive[offset+12:]))
	indexStart := offset + 16
	storeStart := indexStart + entryCount*indexEntryLength
	length := 16 + entryCount*indexEntryLength + storeLength
	if offset+length > len(archive) {
		return header{}, errors.New("an rpm header claims more bytes than the file holds")
	}
	store := archive[storeStart : storeStart+storeLength]
	parsed := header{entries: map[int]headerEntry{}, length: length}
	for index := 0; index < entryCount; index++ {
		record := archive[indexStart+index*indexEntryLength:]
		entry := headerEntry{
			tag:   int(binary.BigEndian.Uint32(record[0:])),
			kind:  int(binary.BigEndian.Uint32(record[4:])),
			count: int(binary.BigEndian.Uint32(record[12:])),
		}
		entryOffset := int(int32(binary.BigEndian.Uint32(record[8:])))
		if entry.tag == signatureRegionTag || entry.tag == immutableRegionTag {
			continue
		}
		data, errorValue := entryData(store, entryOffset, entry)
		if errorValue != nil {
			return header{}, fmt.Errorf("rpm tag %d: %w", entry.tag, errorValue)
		}
		entry.data = data
		parsed.entries[entry.tag] = entry
	}
	return parsed, nil
}

func entryData(store []byte, offset int, entry headerEntry) ([]byte, error) {
	if offset < 0 || offset > len(store) {
		return nil, errors.New("offset outside the header store")
	}
	remaining := store[offset:]
	var length int
	switch entry.kind {
	case typeCharacter, typeInteger8, typeBinary:
		length = entry.count
	case typeInteger16:
		length = 2 * entry.count
	case typeInteger32:
		length = 4 * entry.count
	case typeInteger64:
		length = 8 * entry.count
	case typeString, typeStringArray, typeInternationalis:
		strings := entry.count
		if entry.kind == typeString {
			strings = 1
		}
		for ; strings > 0; strings-- {
			end := bytes.IndexByte(remaining[length:], 0)
			if end < 0 {
				return nil, errors.New("unterminated string")
			}
			length += end + 1
		}
	default:
		return nil, fmt.Errorf("unknown type %d", entry.kind)
	}
	if length > len(remaining) {
		return nil, errors.New("value runs past the header store")
	}
	return remaining[:length], nil
}

func alignment(kind int) int {
	switch kind {
	case typeInteger16:
		return 2
	case typeInteger32:
		return 4
	case typeInteger64:
		return 8
	}
	return 1
}

func encodeRegionHeader(regionTag int, entries map[int]headerEntry) []byte {
	tags := make([]int, 0, len(entries))
	for tag := range entries {
		tags = append(tags, tag)
	}
	sort.Ints(tags)

	var store bytes.Buffer
	offsets := make([]int, len(tags))
	for index, tag := range tags {
		entry := entries[tag]
		if remainder := store.Len() % alignment(entry.kind); remainder != 0 {
			store.Write(make([]byte, alignment(entry.kind)-remainder))
		}
		offsets[index] = store.Len()
		store.Write(entry.data)
	}
	regionEntryCount := len(tags) + 1
	trailer := make([]byte, indexEntryLength)
	binary.BigEndian.PutUint32(trailer[0:], uint32(regionTag))
	binary.BigEndian.PutUint32(trailer[4:], typeBinary)
	binary.BigEndian.PutUint32(trailer[8:], uint32(-int32(indexEntryLength*regionEntryCount)))
	binary.BigEndian.PutUint32(trailer[12:], indexEntryLength)
	regionOffset := store.Len()
	store.Write(trailer)

	var encoded bytes.Buffer
	encoded.Write(headerMagic)
	binary.Write(&encoded, binary.BigEndian, []int32{int32(regionEntryCount), int32(store.Len())})
	writeIndexRecord(&encoded, regionTag, typeBinary, regionOffset, indexEntryLength)
	for index, tag := range tags {
		entry := entries[tag]
		writeIndexRecord(&encoded, tag, entry.kind, offsets[index], entry.count)
	}
	encoded.Write(store.Bytes())
	return encoded.Bytes()
}

func writeIndexRecord(destination *bytes.Buffer, tag int, kind int, offset int, count int) {
	binary.Write(destination, binary.BigEndian, []int32{int32(tag), int32(kind), int32(offset), int32(count)})
}
