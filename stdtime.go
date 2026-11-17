package json

import (
	"reflect"
	"time"
	"unsafe"
)

const (
	rfc3339YearDigits  = len("2006")
	rfc3339ZoneLen     = len("Z07:00")
	rfc3339ZoneHourLen = len("07:00")
	rfc3339MaxZoneHour = 24
)

var (
	timeType              = reflect.TypeFor[time.Time]()
	timePtrType           = reflect.TypeFor[*time.Time]()
	encodeTimeByMarshaler = encodeMarshaler(timeType)
)

func encodeTime(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	start := len(b)
	b = (*time.Time)(p).AppendFormat(append(b, '"'), time.RFC3339Nano)
	if !isStrictRFC3339(b[start+1:]) {
		return encodeTimeByMarshaler(e, b[:start], p)
	}
	return append(b, '"'), nil
}

func encodeTimePointer(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	return encodeTime(e, b, *(*unsafe.Pointer)(p))
}

func isStrictRFC3339(s []byte) bool {
	if s[rfc3339YearDigits] != '-' {
		return false
	}
	if s[len(s)-1] == 'Z' {
		return true
	}
	if c := s[len(s)-rfc3339ZoneLen]; '0' <= c && c <= '9' {
		return false
	}
	hour := s[len(s)-rfc3339ZoneHourLen:]
	return 10*(hour[0]-'0')+(hour[1]-'0') < rfc3339MaxZoneHour
}
