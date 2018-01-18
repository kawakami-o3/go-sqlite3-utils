package varint

func toBigEndian(v uint64) []byte {
	if v == 0 {
		return []byte{0}
	}

	ret := []byte{}
	for v > 0 {
		ret = append(ret, byte(v%256))
		v /= 256
	}
	return ret
}

func Encode(v uint64) []byte {

	if v <= 240 {
		return []byte{byte(v)}
	} else if v <= 2287 {
		return []byte{byte((v-240)/256) + 241, byte(v - 240%256)}
	} else if v <= 67823 {
		return []byte{249, byte((v - 2288) / 256), byte(v - 2288%256)}
	} else if v <= 16777215 {
		return append([]byte{250}, toBigEndian(v)...)
	} else if v <= 4294967295 {
		return append([]byte{251}, toBigEndian(v)...)
	} else if v <= 1099511627775 {
		return append([]byte{252}, toBigEndian(v)...)
	} else if v <= 281474976710655 {
		return append([]byte{253}, toBigEndian(v)...)
	} else if v <= 72057594037927935 {
		return append([]byte{254}, toBigEndian(v)...)
	} else {
		return append([]byte{255}, toBigEndian(v)...)
	}

}
