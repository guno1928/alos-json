package json

import (
	"reflect"
	"strings"
	"sync"
	"unsafe"
)

type fieldLocation struct {
	offset uintptr
	direct bool
	index  []int
}

type structField struct {
	name       string
	foldedName string
	typ        reflect.Type
	fieldLocation
	omitZero    bool
	omitEmpty   bool
	asString    bool
	nocase      bool
	plainDecode bool
	tagged      bool
}

type inlineFallback struct {
	typ reflect.Type
	fieldLocation
}

type structFields struct {
	list      []structField
	nameIdx   map[string]int
	fallback  *inlineFallback
	hasNocase bool
}

var fieldCache sync.Map

func cachedFields(t reflect.Type) *structFields {
	if v, ok := fieldCache.Load(t); ok {
		return v.(*structFields)
	}
	fs := resolveFields(t)
	v, _ := fieldCache.LoadOrStore(t, fs)
	return v.(*structFields)
}

func resolveFields(t reflect.Type) *structFields {
	var candidates []structField
	var fallbacks []inlineFallback
	collectFields(&candidates, &fallbacks, t, nil, map[reflect.Type]bool{t: true})
	list := dominantFields(candidates)
	fs := &structFields{
		list:     list,
		nameIdx:  make(map[string]int, len(list)),
		fallback: dominantFallback(fallbacks),
	}
	for i := range fs.list {
		sf := &fs.list[i]
		sf.locate(t)
		sf.foldedName = strings.ToLower(sf.name)
		sf.plainDecode = !hasCustomUnmarshaler(sf.typ)
		fs.hasNocase = fs.hasNocase || sf.nocase
		fs.nameIdx[sf.name] = i
	}
	if fs.fallback != nil {
		fs.fallback.locate(t)
	}
	return fs
}

func (l *fieldLocation) locate(t reflect.Type) {
	if !containsPointer(t, l.index[:len(l.index)-1]) {
		l.offset = computeOffset(t, l.index)
		l.direct = true
	}
}

func (l *fieldLocation) existing(t reflect.Type, p unsafe.Pointer) (unsafe.Pointer, bool) {
	if l.direct {
		return unsafe.Add(p, l.offset), true
	}
	fv := fieldByIndex(reflect.NewAt(t, p).Elem(), l.index)
	if !fv.IsValid() {
		return nil, false
	}
	return fv.Addr().UnsafePointer(), true
}

func (l *fieldLocation) allocated(t reflect.Type, p unsafe.Pointer) (unsafe.Pointer, bool) {
	if l.direct {
		return unsafe.Add(p, l.offset), true
	}
	fv := fieldByIndexAlloc(reflect.NewAt(t, p).Elem(), l.index)
	if !fv.IsValid() || !fv.CanSet() {
		return nil, false
	}
	return fv.Addr().UnsafePointer(), true
}

func dominantFallback(fallbacks []inlineFallback) *inlineFallback {
	var best *inlineFallback
	ambiguous := false
	for i := range fallbacks {
		fb := &fallbacks[i]
		switch {
		case best == nil || len(fb.index) < len(best.index):
			best, ambiguous = fb, false
		case len(fb.index) == len(best.index):
			ambiguous = true
		}
	}
	if ambiguous {
		return nil
	}
	return best
}

func isInlineFallbackType(t reflect.Type) bool {
	return t.Kind() == reflect.Map && t.Key().Kind() == reflect.String
}

func hasCustomUnmarshaler(t reflect.Type) bool {
	pt := reflect.PointerTo(t)
	return pt.Implements(unmarshalerFromType) || pt.Implements(unmarshalerType) || pt.Implements(textUnmarshalerType) || isStdNumberType(t)
}

func collectFields(out *[]structField, fallbacks *[]inlineFallback, t reflect.Type, index []int, visiting map[reflect.Type]bool) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, opts := parseTag(tag)
		inline := hasOption(opts, "inline")
		fieldIndex := append(append([]int(nil), index...), i)

		if inline && isInlineFallbackType(f.Type) {
			if f.IsExported() {
				*fallbacks = append(*fallbacks, inlineFallback{fieldLocation: fieldLocation{index: fieldIndex}, typ: f.Type})
			}
			continue
		}
		if f.Anonymous || inline {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct && (inline || name == "") {
				if (!f.IsExported() && !f.Anonymous) || visiting[ft] {
					continue
				}
				visiting[ft] = true
				collectFields(out, fallbacks, ft, fieldIndex, visiting)
				delete(visiting, ft)
				continue
			}
		}
		if !f.IsExported() {
			continue
		}

		sf := structField{
			fieldLocation: fieldLocation{index: fieldIndex},
			typ:           f.Type,
			name:          name,
			tagged:        name != "",
		}
		if name == "" {
			sf.name = f.Name
		}
		for _, opt := range opts {
			switch opt {
			case "omitzero":
				sf.omitZero = true
			case "omitempty":
				sf.omitEmpty = true
			case "string":
				sf.asString = true
			case "nocase":
				sf.nocase = true
			}
		}
		*out = append(*out, sf)
	}
}

func hasOption(opts []string, want string) bool {
	for _, opt := range opts {
		if opt == want {
			return true
		}
	}
	return false
}

func dominantFields(candidates []structField) []structField {
	byName := make(map[string][]int, len(candidates))
	for i := range candidates {
		byName[candidates[i].name] = append(byName[candidates[i].name], i)
	}
	keep := make([]bool, len(candidates))
	for _, idxs := range byName {
		if len(idxs) == 1 {
			keep[idxs[0]] = true
			continue
		}
		minDepth := len(candidates[idxs[0]].index)
		for _, i := range idxs[1:] {
			minDepth = min(minDepth, len(candidates[i].index))
		}
		shallowest, taggedShallowest := -1, -1
		shallowCount, taggedCount := 0, 0
		for _, i := range idxs {
			if len(candidates[i].index) != minDepth {
				continue
			}
			shallowCount++
			shallowest = i
			if candidates[i].tagged {
				taggedCount++
				taggedShallowest = i
			}
		}
		switch {
		case shallowCount == 1:
			keep[shallowest] = true
		case taggedCount == 1:
			keep[taggedShallowest] = true
		}
	}
	out := make([]structField, 0, len(candidates))
	for i := range candidates {
		if keep[i] {
			out = append(out, candidates[i])
		}
	}
	return out
}

func containsPointer(t reflect.Type, index []int) bool {
	for _, i := range index {
		if t.Kind() == reflect.Pointer {
			return true
		}
		f := t.Field(i)
		t = f.Type
	}
	return t.Kind() == reflect.Pointer
}

func computeOffset(t reflect.Type, index []int) uintptr {
	var off uintptr
	for _, i := range index {
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		f := t.Field(i)
		off += f.Offset
		t = f.Type
	}
	return off
}

func parseTag(tag string) (string, []string) {
	if tag == "" {
		return "", nil
	}
	parts := strings.Split(tag, ",")
	name := parts[0]
	if len(parts) > 1 {
		return name, parts[1:]
	}
	return name, nil
}

func (fs *structFields) fieldByName(name string, caseInsensitive bool) (structField, bool) {
	if i := fs.indexOf(name, caseInsensitive); i >= 0 {
		return fs.list[i], true
	}
	return structField{}, false
}

func (fs *structFields) indexOf(name string, caseInsensitive bool) int {
	if i, ok := fs.nameIdx[name]; ok {
		return i
	}
	if !caseInsensitive && !fs.hasNocase {
		return -1
	}
	folded := strings.ToLower(name)
	for i := range fs.list {
		sf := &fs.list[i]
		if (caseInsensitive || sf.nocase) && sf.foldedName == folded {
			return i
		}
	}
	return -1
}
