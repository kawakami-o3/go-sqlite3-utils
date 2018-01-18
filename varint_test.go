package varint

import (
	"testing"
)

func assertEq(t *testing.T, expect, actual byte) {
	if expect != actual {
		t.Error("Expected:", expect, "Actual:", actual)
	}
}

func assertEqAll(t *testing.T, expect byte, actual []byte) {

	for i, a := range actual {
		if expect != a {
			t.Error("Expected", expect, "Actual", a, "at", i, "in", actual)
		}
	}

}

func TestEncodeMax(t *testing.T) {

	t.Run("v=240", func(t *testing.T) {
		bytes := Encode(240)
		//fmt.Println(bytes)
		assertEq(t, 240, bytes[0])
	})

	testCase := map[uint64]byte{
		2287:                 248,
		67823:                249,
		16777215:             250,
		4294967295:           251,
		1099511627775:        252,
		281474976710655:      253,
		72057594037927935:    254,
		18446744073709551615: 255,
	}

	for k, v := range testCase {
		t.Run("v="+string(k), func(t *testing.T) {
			bytes := Encode(k)
			//fmt.Println(v, bytes)
			assertEq(t, v, bytes[0])
			assertEqAll(t, 255, bytes[1:])
		})
	}
}
