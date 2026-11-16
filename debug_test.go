package json

import (
	"math"
	"testing"
)

func TestDebugRoundtrip(t *testing.T) {
	type AllP struct {
		B  bool    `json:"b"`
		I  int     `json:"i"`
		I8 int8    `json:"i8"`
		I6 int64   `json:"i6"`
		U  uint    `json:"u"`
		U8 uint8   `json:"u8"`
		U6 uint64  `json:"u6"`
		F3 float32 `json:"f3"`
		F6 float64 `json:"f6"`
		S  string  `json:"s"`
	}
	pin := AllP{B: true, I: -1, I8: -128, I6: math.MinInt64, U: 1, U8: 255,
		U6: math.MaxUint64, F3: 3.14, F6: 2.718281828459045, S: "test"}
	b, err := Marshal(pin)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("JSON: %s", b)
	var pout AllP
	if err := Unmarshal(b, &pout); err != nil {
		t.Fatal(err)
	}
	if pout.U6 != pin.U6 {
		t.Errorf("U6: want %d got %d", pin.U6, pout.U6)
	}
	if pout.F6 != pin.F6 {
		t.Errorf("F6: want %v got %v", pin.F6, pout.F6)
	}
	if pout.S != pin.S {
		t.Errorf("S: want %q got %q", pin.S, pout.S)
	}
	if pout.I6 != pin.I6 {
		t.Errorf("I6: want %d got %d", pin.I6, pout.I6)
	}
	if pout.I != pin.I {
		t.Errorf("I: want %d got %d", pin.I, pout.I)
	}
	if pout.I8 != pin.I8 {
		t.Errorf("I8: want %d got %d", pin.I8, pout.I8)
	}
}
