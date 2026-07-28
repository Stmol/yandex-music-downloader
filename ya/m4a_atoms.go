package ya

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/tommyo123/mtag/mp4"
)

// ensureM4AMetadataTree creates the iTunes metadata path expected by mtag
// when an otherwise valid M4A has no moov/udta/meta/ilst atom yet.
func ensureM4AMetadataTree(path string) error {
	source, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open M4A for metadata bootstrap: %w", err)
	}
	sourceClosed := false
	closeSource := func() error {
		if sourceClosed {
			return nil
		}
		sourceClosed = true
		return source.Close()
	}
	defer func() {
		_ = closeSource()
	}()

	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("stat M4A for metadata bootstrap: %w", err)
	}
	atoms := mp4.WalkTopLevel(source, info.Size())
	if err := validateMP4TopLevelAtoms(atoms, info.Size()); err != nil {
		return err
	}

	moovIndex := indexMP4TopLevelAtom(atoms, "moov")
	if moovIndex < 0 {
		return fmt.Errorf("M4A metadata bootstrap: MP4 file has no moov atom")
	}

	oldMoov := atoms[moovIndex]
	moovBody := make([]byte, oldMoov.DataSize)
	if _, err := source.ReadAt(moovBody, oldMoov.DataAt); err != nil {
		return fmt.Errorf("read M4A moov atom: %w", err)
	}
	newMoovBody, changed, err := bootstrapM4AMetadataTree(moovBody)
	if err != nil {
		return err
	}
	if !changed {
		if err := closeSource(); err != nil {
			return fmt.Errorf("close M4A after metadata bootstrap check: %w", err)
		}
		return nil
	}

	newMoov, err := buildMP4Atom("moov", newMoovBody)
	if err != nil {
		return err
	}
	mdatBefore, mdatAfter := mp4MediaDataAroundMoov(atoms, moovIndex)
	if mdatBefore && mdatAfter {
		return fmt.Errorf("M4A metadata bootstrap: unsupported MP4 with mdat atoms both before and after moov")
	}
	if mdatAfter {
		delta := int64(len(newMoov)) - oldMoov.Size
		if delta != 0 {
			mp4.PatchSampleOffsets(newMoovBody, delta)
			newMoov, err = buildMP4Atom("moov", newMoovBody)
			if err != nil {
				return err
			}
		}
	}

	if err := rewriteM4AWithMoov(source, info, atoms, moovIndex, newMoov, closeSource); err != nil {
		return err
	}
	return nil
}

func mp4MediaDataAroundMoov(atoms []mp4.TopLevelAtom, moovIndex int) (before, after bool) {
	for i, atom := range atoms {
		if string(atom.Name[:]) != "mdat" {
			continue
		}
		if i < moovIndex {
			before = true
		} else if i > moovIndex {
			after = true
		}
	}
	return before, after
}

func validateMP4TopLevelAtoms(atoms []mp4.TopLevelAtom, fileSize int64) error {
	if fileSize < 8 || len(atoms) == 0 {
		return fmt.Errorf("M4A metadata bootstrap: malformed MP4 top-level atoms")
	}
	var cursor int64
	for _, atom := range atoms {
		if atom.Offset != cursor || atom.Size < 8 || atom.DataAt < atom.Offset || atom.DataSize < 0 || atom.Offset+atom.Size > fileSize {
			return fmt.Errorf("M4A metadata bootstrap: malformed MP4 top-level atom")
		}
		cursor += atom.Size
	}
	if cursor != fileSize {
		return fmt.Errorf("M4A metadata bootstrap: malformed MP4 top-level atoms")
	}
	return nil
}

func indexMP4TopLevelAtom(atoms []mp4.TopLevelAtom, name string) int {
	for i, atom := range atoms {
		if string(atom.Name[:]) == name {
			return i
		}
	}
	return -1
}

func bootstrapM4AMetadataTree(moovBody []byte) ([]byte, bool, error) {
	udta, ok, err := findMP4ChildAtom(moovBody, "udta")
	if err != nil {
		return nil, false, fmt.Errorf("M4A metadata bootstrap: parse moov: %w", err)
	}
	if !ok {
		metadata, err := newM4AMetadataAtom()
		if err != nil {
			return nil, false, err
		}
		newUDTABody, err := appendMP4Atom(nil, "meta", metadata)
		if err != nil {
			return nil, false, err
		}
		newMoovBody, err := appendMP4Atom(moovBody, "udta", newUDTABody)
		if err != nil {
			return nil, false, err
		}
		return newMoovBody, true, nil
	}

	udtaBody := moovBody[udta.headerSize+udta.offset : udta.end]
	meta, ok, err := findMP4ChildAtom(udtaBody, "meta")
	if err != nil {
		return nil, false, fmt.Errorf("M4A metadata bootstrap: parse udta: %w", err)
	}
	if !ok {
		metadata, err := newM4AMetadataAtom()
		if err != nil {
			return nil, false, err
		}
		newUDTA, err := appendMP4Atom(udtaBody, "meta", metadata)
		if err != nil {
			return nil, false, err
		}
		return replaceMP4Atom(moovBody, udta, "udta", newUDTA)
	}

	metaBody := udtaBody[meta.offset+meta.headerSize : meta.end]
	if len(metaBody) < 4 {
		return nil, false, fmt.Errorf("M4A metadata bootstrap: meta atom is too short for full-atom prefix")
	}
	if _, ok, err := findMP4ChildAtom(metaBody[4:], "ilst"); err != nil {
		return nil, false, fmt.Errorf("M4A metadata bootstrap: parse meta: %w", err)
	} else if ok {
		return moovBody, false, nil
	}

	newMetaChildren, err := appendMP4Atom(metaBody[4:], "ilst", nil)
	if err != nil {
		return nil, false, err
	}
	newMeta := append(append([]byte{}, metaBody[:4]...), newMetaChildren...)
	newUDTA, _, err := replaceMP4Atom(udtaBody, meta, "meta", newMeta)
	if err != nil {
		return nil, false, err
	}
	return replaceMP4Atom(moovBody, udta, "udta", newUDTA)
}

func newM4AMetadataAtom() ([]byte, error) {
	hdlr, err := buildMP4Atom("hdlr", []byte{
		0, 0, 0, 0, // version and flags
		0, 0, 0, 0, // pre-defined
		'm', 'd', 'i', 'r',
		'a', 'p', 'p', 'l',
		0, 0, 0, 0, // component flags
		0, 0, 0, 0, // component flags mask
		0, // empty component name
	})
	if err != nil {
		return nil, err
	}
	ilst, err := buildMP4Atom("ilst", nil)
	if err != nil {
		return nil, err
	}
	metaBody := make([]byte, 0, 4+len(hdlr)+len(ilst))
	metaBody = append(metaBody, 0, 0, 0, 0) // full-atom version and flags
	metaBody = append(metaBody, hdlr...)
	metaBody = append(metaBody, ilst...)
	return metaBody, nil
}

type m4aAtomLocation struct {
	offset     int
	end        int
	headerSize int
	sizeToEnd  bool
	typ        string
}

func findMP4ChildAtom(body []byte, target string) (m4aAtomLocation, bool, error) {
	for offset := 0; offset < len(body); {
		atom, err := parseM4AAtom(body, offset)
		if err != nil {
			return m4aAtomLocation{}, false, err
		}
		if atom.typ == target {
			return atom, true, nil
		}
		offset = atom.end
	}
	return m4aAtomLocation{}, false, nil
}

func parseM4AAtom(data []byte, offset int) (m4aAtomLocation, error) {
	if offset < 0 || len(data)-offset < 8 {
		return m4aAtomLocation{}, fmt.Errorf("truncated atom header at offset %d", offset)
	}
	size := uint64(binary.BigEndian.Uint32(data[offset : offset+4]))
	headerSize := 8
	sizeToEnd := false
	switch size {
	case 0:
		sizeToEnd = true
		size = uint64(len(data) - offset)
	case 1:
		if len(data)-offset < 16 {
			return m4aAtomLocation{}, fmt.Errorf("truncated 64-bit atom header at offset %d", offset)
		}
		size = binary.BigEndian.Uint64(data[offset+8 : offset+16])
		headerSize = 16
	}
	if size < uint64(headerSize) || size > uint64(len(data)-offset) || size > math.MaxInt {
		return m4aAtomLocation{}, fmt.Errorf("invalid atom size at offset %d", offset)
	}
	end := offset + int(size)
	return m4aAtomLocation{
		offset:     offset,
		end:        end,
		headerSize: headerSize,
		sizeToEnd:  sizeToEnd,
		typ:        string(data[offset+4 : offset+8]),
	}, nil
}

func appendMP4Atom(body []byte, typ string, payload []byte) ([]byte, error) {
	var err error
	body, err = normalizeTerminalMP4Atom(body)
	if err != nil {
		return nil, err
	}
	atom, err := buildMP4Atom(typ, payload)
	if err != nil {
		return nil, err
	}
	result := make([]byte, 0, len(body)+len(atom))
	result = append(result, body...)
	return append(result, atom...), nil
}

// normalizeTerminalMP4Atom converts an atom whose size extends to the end of
// its parent (size == 0) into an explicit-size atom before appending a
// sibling. Otherwise the new sibling would remain inside the terminal atom.
func normalizeTerminalMP4Atom(body []byte) ([]byte, error) {
	for offset := 0; offset < len(body); {
		atom, err := parseM4AAtom(body, offset)
		if err != nil {
			return nil, err
		}
		if atom.sizeToEnd {
			explicit, err := buildMP4Atom(atom.typ, body[atom.offset+atom.headerSize:atom.end])
			if err != nil {
				return nil, err
			}
			result := make([]byte, 0, len(body))
			result = append(result, body[:atom.offset]...)
			return append(result, explicit...), nil
		}
		offset = atom.end
	}
	return body, nil
}

func replaceMP4Atom(body []byte, old m4aAtomLocation, typ string, payload []byte) ([]byte, bool, error) {
	atom, err := buildMP4Atom(typ, payload)
	if err != nil {
		return nil, false, err
	}
	result := make([]byte, 0, len(body)-old.end+old.offset+len(atom))
	result = append(result, body[:old.offset]...)
	result = append(result, atom...)
	result = append(result, body[old.end:]...)
	return result, true, nil
}

func buildMP4Atom(typ string, payload []byte) ([]byte, error) {
	if len(typ) != 4 {
		return nil, fmt.Errorf("M4A metadata bootstrap: invalid atom type %q", typ)
	}
	if len(payload) > math.MaxUint32-8 {
		return nil, fmt.Errorf("M4A metadata bootstrap: atom %s is too large", typ)
	}
	atom := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(atom[:4], uint32(len(atom)))
	copy(atom[4:8], typ)
	copy(atom[8:], payload)
	return atom, nil
}

func rewriteM4AWithMoov(source *os.File, info os.FileInfo, atoms []mp4.TopLevelAtom, moovIndex int, newMoov []byte, closeSource func() error) error {
	dir := filepath.Dir(source.Name())
	base := filepath.Base(source.Name())
	temp, err := os.CreateTemp(dir, "."+base+".metadata-*")
	if err != nil {
		return fmt.Errorf("create M4A metadata temporary file: %w", err)
	}
	tempPath := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}
	if err := temp.Chmod(info.Mode()); err != nil {
		cleanup()
		return fmt.Errorf("set M4A metadata temporary file permissions: %w", err)
	}

	for i, atom := range atoms {
		if i == moovIndex {
			if _, err := temp.Write(newMoov); err != nil {
				cleanup()
				return fmt.Errorf("write M4A metadata moov atom: %w", err)
			}
			continue
		}
		if _, err := source.Seek(atom.Offset, io.SeekStart); err != nil {
			cleanup()
			return fmt.Errorf("seek M4A atom for metadata rewrite: %w", err)
		}
		if _, err := io.CopyN(temp, source, atom.Size); err != nil {
			cleanup()
			return fmt.Errorf("copy M4A atom for metadata rewrite: %w", err)
		}
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close M4A metadata temporary file: %w", err)
	}
	if err := closeSource(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close M4A before metadata replacement: %w", err)
	}
	if err := os.Rename(tempPath, source.Name()); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace M4A after metadata bootstrap: %w", err)
	}
	return nil
}
