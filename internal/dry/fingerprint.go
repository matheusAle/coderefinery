package dry

import (
	"encoding/binary"
	"slices"
	"strings"
)

// Fingerprinter interns the fingerprints of one language (spec-dryer §4.4)
// into dense uint32 ids, so an entry carries a sorted []uint32 instead of a
// set of printed strings and pairing compares integers.
//
// The default mode never prints a fingerprint. An atom's key is its kind
// and text; a list's key is the ids of its elements. Two keys are equal
// exactly when the printed forms are equal, because pr is injective as long
// as no keyword name contains a space or "]" (strings are escaped, and a
// keyword token ends at the first space or "]"). Node types and dry4clj's
// tags never do; a keyword that would breaks the equivalence sets
// ambiguous, and ScanFiles then redoes that language by printed string.
type Fingerprinter struct {
	ids       map[string]uint32
	byString  bool
	ambiguous bool
	stack     []uint32 // element ids of the lists being walked
	key       []byte   // scratch key
	set       []uint32 // scratch id list of the current value
}

// Key tags: an atom key is tag+text, a list key is tagList+element ids.
// The tags keep the three key spaces apart.
const (
	tagKeyword byte = iota
	tagString
	tagList
)

// NewFingerprinter returns an empty interner. byString keys every
// fingerprint by its printed form (the reference definition) instead of by
// structure.
func NewFingerprinter(byString bool) *Fingerprinter {
	return &Fingerprinter{ids: map[string]uint32{}, byString: byString}
}

// Set is Fingerprints(v) as sorted, distinct ids.
func (f *Fingerprinter) Set(v *Value) []uint32 {
	if f.byString {
		return f.strings(Fingerprints(v))
	}
	f.set = f.set[:0]
	f.walk(v)
	return sortedUnique(f.set)
}

// strings interns a set of printed fingerprints.
func (f *Fingerprinter) strings(fps map[string]struct{}) []uint32 {
	f.set = f.set[:0]
	for s := range fps {
		f.set = append(f.set, f.intern(s))
	}
	return sortedUnique(f.set)
}

// walk interns v and every subtree of it, appends their ids to f.set and
// returns v's id.
func (f *Fingerprinter) walk(v *Value) uint32 {
	var id uint32
	if v.Kind == KindList {
		base := len(f.stack)
		for _, item := range v.Items {
			f.stack = append(f.stack, f.walk(item))
		}
		f.key = append(f.key[:0], tagList)
		for _, e := range f.stack[base:] {
			f.key = binary.LittleEndian.AppendUint32(f.key, e)
		}
		f.stack = f.stack[:base]
		id = f.internKey()
	} else {
		tag := tagString
		if v.Kind == KindKeyword {
			tag = tagKeyword
			if strings.ContainsAny(v.Text, " ]") {
				f.ambiguous = true
			}
		}
		f.key = append(append(f.key[:0], tag), v.Text...)
		id = f.internKey()
	}
	f.set = append(f.set, id)
	return id
}

// internKey is intern(string(f.key)) without allocating on a hit.
func (f *Fingerprinter) internKey() uint32 {
	if id, ok := f.ids[string(f.key)]; ok {
		return id
	}
	return f.intern(string(f.key))
}

func (f *Fingerprinter) intern(key string) uint32 {
	if id, ok := f.ids[key]; ok {
		return id
	}
	id := uint32(len(f.ids))
	f.ids[key] = id
	return id
}

// sortedUnique returns a right-sized sorted copy of ids without duplicates.
func sortedUnique(ids []uint32) []uint32 {
	slices.Sort(ids)
	return slices.Clone(slices.Compact(ids))
}
