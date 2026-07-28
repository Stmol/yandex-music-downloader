package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

type atom struct {
	start      int
	end        int
	size       int
	headerSize int
	typ        string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "generate m4a fixture: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: %s <co64|strip-metadata|move-moov-before-mdat|truncate-trkn> <input> <output>", os.Args[0])
	}

	command := args[0]
	inputPath := args[1]
	outputPath := args[2]

	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	switch command {
	case "co64":
		data, err = rewriteChunkOffsetsAsCO64(data)
	case "strip-metadata":
		data, err = stripMetadataTree(data)
	case "move-moov-before-mdat":
		data, err = moveMoovBeforeMdat(data)
	case "truncate-trkn":
		data, err = truncateTrackAtom(data)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
	if err != nil {
		return err
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func stripMetadataTree(data []byte) ([]byte, error) {
	moov, ok, err := findTopLevelAtom(data, "moov")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("moov atom not found")
	}

	moovBody := data[moov.start+moov.headerSize : moov.end]
	udta, ok, err := findDirectChildAtom(moovBody, "udta")
	if err != nil {
		return nil, err
	}
	if !ok {
		return append([]byte(nil), data...), nil
	}

	newMoovBody := append([]byte{}, moovBody[:udta.start]...)
	newMoovBody = append(newMoovBody, moovBody[udta.end:]...)
	newMoov := buildAtom("moov", newMoovBody)
	result := make([]byte, 0, len(data)-moov.size+len(newMoov))
	result = append(result, data[:moov.start]...)
	result = append(result, newMoov...)
	result = append(result, data[moov.end:]...)
	return result, nil
}

func moveMoovBeforeMdat(data []byte) ([]byte, error) {
	moov, ok, err := findTopLevelAtom(data, "moov")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("moov atom not found")
	}
	mdat, ok, err := findTopLevelAtom(data, "mdat")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("mdat atom not found")
	}
	if moov.start < mdat.start {
		return nil, errors.New("moov already precedes mdat")
	}

	moovBody := append([]byte(nil), data[moov.start+moov.headerSize:moov.end]...)
	// Moving moov immediately before mdat shifts mdat by the complete moov
	// atom size, so every media chunk offset must move by that amount too.
	patchSampleOffsets(moovBody, int64(moov.size))
	patchedMoov := buildAtom("moov", moovBody)
	result := make([]byte, 0, len(data))
	result = append(result, data[:mdat.start]...)
	result = append(result, patchedMoov...)
	result = append(result, data[mdat.start:moov.start]...)
	result = append(result, data[moov.end:]...)
	return result, nil
}

func rewriteChunkOffsetsAsCO64(data []byte) ([]byte, error) {
	mdat, ok, err := findAtom(data, "", "mdat")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("mdat atom not found")
	}

	stco, ok, err := findAtom(data, "", "stco")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("stco atom not found")
	}
	if mdat.start > stco.end {
		return nil, errors.New("stco->co64 rewrite requires source with mdat before moov; generate the source without faststart")
	}

	rewrote := false
	rebuilt, err := rebuildAtoms(data, "", func(typ string, payload []byte) ([]byte, bool, error) {
		if typ != "stco" {
			return nil, false, nil
		}

		if len(payload) < 8 {
			return nil, false, fmt.Errorf("stco payload too short: %d", len(payload))
		}

		entryCount := int(binary.BigEndian.Uint32(payload[4:8]))
		expectedLen := 8 + entryCount*4
		if len(payload) != expectedLen {
			return nil, false, fmt.Errorf("stco payload length mismatch: got %d want %d", len(payload), expectedLen)
		}

		co64Payload := make([]byte, 8+entryCount*8)
		copy(co64Payload[:8], payload[:8])
		for i := 0; i < entryCount; i++ {
			offset := binary.BigEndian.Uint32(payload[8+i*4 : 12+i*4])
			binary.BigEndian.PutUint64(co64Payload[8+i*8:16+i*8], uint64(offset))
		}

		rewrote = true
		return buildAtom("co64", co64Payload), true, nil
	})
	if err != nil {
		return nil, err
	}
	if !rewrote {
		return nil, errors.New("stco atom not rewritten")
	}
	return rebuilt, nil
}

func truncateTrackAtom(data []byte) ([]byte, error) {
	trkn, ok, err := findAtom(data, "", "trkn")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("trkn atom not found")
	}
	if trkn.size <= 10 {
		return nil, fmt.Errorf("trkn atom too small to truncate safely: %d", trkn.size)
	}

	cutAt := trkn.end - 2
	if cutAt <= trkn.start+trkn.headerSize {
		return nil, errors.New("truncation point would remove the full trkn payload")
	}

	return append([]byte(nil), data[:cutAt]...), nil
}

func rebuildAtoms(data []byte, parentType string, rewrite func(typ string, payload []byte) ([]byte, bool, error)) ([]byte, error) {
	out := make([]byte, 0, len(data))
	for offset := 0; offset < len(data); {
		current, err := parseAtom(data, offset)
		if err != nil {
			return nil, err
		}

		payload := data[offset+current.headerSize : current.end]
		if replacement, ok, err := rewrite(current.typ, payload); err != nil {
			return nil, err
		} else if ok {
			out = append(out, replacement...)
			offset = current.end
			continue
		}

		if isContainer(parentType, current.typ) {
			prefixLen := 0
			switch current.typ {
			case "meta":
				if len(payload) < 4 {
					return nil, fmt.Errorf("meta atom payload too short: %d", len(payload))
				}
				prefixLen = 4
			}

			rebuiltChildren, err := rebuildAtoms(payload[prefixLen:], current.typ, rewrite)
			if err != nil {
				return nil, err
			}

			rebuiltPayload := append([]byte{}, payload[:prefixLen]...)
			rebuiltPayload = append(rebuiltPayload, rebuiltChildren...)
			out = append(out, buildAtom(current.typ, rebuiltPayload)...)
		} else {
			out = append(out, data[offset:current.end]...)
		}

		offset = current.end
	}

	return out, nil
}

func findAtom(data []byte, parentType string, targetType string) (atom, bool, error) {
	return findAtomFrom(data, 0, parentType, targetType)
}

func findTopLevelAtom(data []byte, targetType string) (atom, bool, error) {
	for offset := 0; offset < len(data); {
		current, err := parseAtom(data, offset)
		if err != nil {
			return atom{}, false, err
		}
		if current.typ == targetType {
			return current, true, nil
		}
		offset = current.end
	}
	return atom{}, false, nil
}

func findDirectChildAtom(data []byte, targetType string) (atom, bool, error) {
	for offset := 0; offset < len(data); {
		current, err := parseAtom(data, offset)
		if err != nil {
			return atom{}, false, err
		}
		if current.typ == targetType {
			return current, true, nil
		}
		offset = current.end
	}
	return atom{}, false, nil
}

func findAtomFrom(data []byte, baseOffset int, parentType string, targetType string) (atom, bool, error) {
	for offset := 0; offset < len(data); {
		current, err := parseAtom(data, offset)
		if err != nil {
			return atom{}, false, err
		}

		current.start += baseOffset
		current.end += baseOffset

		if current.typ == targetType {
			return current, true, nil
		}

		if isContainer(parentType, current.typ) {
			payload := data[offset+current.headerSize : offset+current.size]
			prefixLen := 0
			if current.typ == "meta" {
				if len(payload) < 4 {
					return atom{}, false, fmt.Errorf("meta atom payload too short: %d", len(payload))
				}
				prefixLen = 4
			}
			found, ok, err := findAtomFrom(payload[prefixLen:], current.start+current.headerSize+prefixLen, current.typ, targetType)
			if err != nil {
				return atom{}, false, err
			}
			if ok {
				return found, true, nil
			}
		}

		offset += current.size
	}

	return atom{}, false, nil
}

func parseAtom(data []byte, offset int) (atom, error) {
	if len(data[offset:]) < 8 {
		return atom{}, fmt.Errorf("truncated atom header at offset %d", offset)
	}

	size := int(binary.BigEndian.Uint32(data[offset : offset+4]))
	typ := string(data[offset+4 : offset+8])
	headerSize := 8

	switch size {
	case 0:
		size = len(data) - offset
	case 1:
		if len(data[offset:]) < 16 {
			return atom{}, fmt.Errorf("truncated 64-bit atom header at offset %d", offset)
		}
		size64 := binary.BigEndian.Uint64(data[offset+8 : offset+16])
		if size64 > uint64(len(data)-offset) {
			return atom{}, fmt.Errorf("64-bit atom %s exceeds file bounds at offset %d", typ, offset)
		}
		size = int(size64)
		headerSize = 16
	}

	if size < headerSize {
		return atom{}, fmt.Errorf("invalid atom size %d for %s at offset %d", size, typ, offset)
	}
	if offset+size > len(data) {
		return atom{}, fmt.Errorf("atom %s at offset %d exceeds file bounds", typ, offset)
	}

	return atom{
		start:      offset,
		end:        offset + size,
		size:       size,
		headerSize: headerSize,
		typ:        typ,
	}, nil
}

func buildAtom(typ string, payload []byte) []byte {
	size := 8 + len(payload)
	out := make([]byte, size)
	binary.BigEndian.PutUint32(out[:4], uint32(size))
	copy(out[4:8], []byte(typ))
	copy(out[8:], payload)
	return out
}

func patchSampleOffsets(body []byte, delta int64) {
	for offset := 0; offset < len(body); {
		current, err := parseAtom(body, offset)
		if err != nil {
			return
		}
		payload := body[offset+current.headerSize : current.end]
		switch current.typ {
		case "stco":
			if len(payload) >= 8 {
				count := int(binary.BigEndian.Uint32(payload[4:8]))
				for i := 0; i < count && 12+i*4 <= len(payload); i++ {
					value := binary.BigEndian.Uint32(payload[8+i*4 : 12+i*4])
					binary.BigEndian.PutUint32(payload[8+i*4:12+i*4], uint32(int64(value)+delta))
				}
			}
		case "co64":
			if len(payload) >= 8 {
				count := int(binary.BigEndian.Uint32(payload[4:8]))
				for i := 0; i < count && 16+i*8 <= len(payload); i++ {
					value := binary.BigEndian.Uint64(payload[8+i*8 : 16+i*8])
					binary.BigEndian.PutUint64(payload[8+i*8:16+i*8], uint64(int64(value)+delta))
				}
			}
		case "moov", "trak", "mdia", "minf", "stbl", "edts", "udta":
			patchSampleOffsets(payload, delta)
		}
		offset = current.end
	}
}

func isContainer(parentType string, typ string) bool {
	if parentType == "ilst" {
		return true
	}

	switch typ {
	case "moov", "trak", "mdia", "minf", "stbl", "udta", "meta", "ilst":
		return true
	default:
		return false
	}
}
