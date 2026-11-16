package json

import (
	"reflect"
	"strconv"
	"sync"
	"testing"
)

const (
	typeCacheStressTypes      = 600
	typeCacheStressGoroutines = 8
	typeCacheStressRounds     = 3
)

func typeCacheStressValues() []reflect.Value {
	values := make([]reflect.Value, typeCacheStressTypes)
	for i := range values {
		typ := reflect.StructOf([]reflect.StructField{
			{Name: "N" + strconv.Itoa(i), Type: reflect.TypeFor[int](), Tag: reflect.StructTag(`json:"n` + strconv.Itoa(i) + `"`)},
			{Name: "S", Type: reflect.TypeFor[string](), Tag: `json:"s"`},
		})
		v := reflect.New(typ)
		v.Elem().Field(0).SetInt(int64(i))
		v.Elem().Field(1).SetString("v" + strconv.Itoa(i))
		values[i] = v
	}
	return values
}

func TestTypeCacheSlotCollisionsConcurrent(t *testing.T) {
	values := typeCacheStressValues()
	var wg sync.WaitGroup
	errs := make(chan error, typeCacheStressGoroutines)
	for g := range typeCacheStressGoroutines {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for round := range typeCacheStressRounds {
				for k := range values {
					i := (k*7 + offset*31 + round) % len(values)
					want := `{"n` + strconv.Itoa(i) + `":` + strconv.Itoa(i) + `,"s":"v` + strconv.Itoa(i) + `"}`
					got, err := Marshal(values[i].Interface())
					if err != nil || string(got) != want {
						errs <- &SemanticError{Msg: "marshal " + strconv.Itoa(i) + ": got " + string(got)}
						return
					}
					back := reflect.New(values[i].Elem().Type())
					if err := Unmarshal(got, back.Interface()); err != nil || !reflect.DeepEqual(back.Elem().Interface(), values[i].Elem().Interface()) {
						errs <- &SemanticError{Msg: "unmarshal " + strconv.Itoa(i)}
						return
					}
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
