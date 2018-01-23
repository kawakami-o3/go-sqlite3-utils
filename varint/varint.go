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

func toUint64(v uint64) []byte {
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

func Decode(bytes []byte) (uint64, uint) {
	if len(bytes) == 0 {
		return 0, 0
	}

	v := uint64(bytes[0])

	if v <= 240 {
		return v, 1
	} else if v <= 248 {
		return 240 + 256*(v-241) + uint64(bytes[1]), 2
	} else if v == 249 {
		return 2288 + 256*uint64(bytes[1]) + uint64(bytes[2]), 3
		//} else if v == 250 {
	} else {
		ret := uint64(0)
		for i := uint64(0); i < v-247; i++ {
			ret = 256*ret + uint64(bytes[i+1])
		}
		return ret, uint(v - 247)
	}

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
