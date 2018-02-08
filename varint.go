package sqlite3utils

import "fmt"

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

func decodeVarint32(bytes []byte) (uint64, uint) {
	fmt.Println(bytes)
	v := uint64(bytes[0])
	consume := uint(1)
	consumeMax := uint(8)
	if v >= 0x80 {
		v &= 0x7f

		for {
			a := uint64(bytes[consume])
			v = (v << 7) | (a & 0x7f)
			consume++

			if !(a >= 0x80 && consume < consumeMax) {
				break
			}
		}
	}
	return v, consume

}

// sqlite3/src/util.c:825
func decodeVarint(bytes []byte) (uint64, uint) {
	v := uint64(bytes[0])
	consume := uint(1)
	consumeMax := uint(8)
	//fmt.Println("varint>>", v, consume)

	if v >= 0x80 {
		v &= 0x7f
		//fmt.Println("varint>>", v, consume)

		for {
			a := uint64(bytes[consume])
			v = (v << 7) | (a & 0x7f)
			//fmt.Println("varint>>", v, consume, a)
			consume++

			if a < 0x80 {
				break
			}
			if consume >= consumeMax-1 {
				a := uint64(bytes[consume])
				v = (v << 8) | a
				//fmt.Println("varint>>", v, consume, a)
				consume++
				break
			}
		}
	}
	return v, consume
}

// sqlite3/src/util.c:759
func encodeVarint(v uint64) []byte {
	if v <= 0x7f {
		return []byte{byte(v & 0x7f)}
	}
	if v <= 0x3fff {
		return []byte{
			byte(((v >> 7) & 0x7f) | 0x80),
			byte(v & 0x7f),
		}
	}

	if (v & (0xff000000 << 32)) != 0 {
		ret := []byte{byte(v)}
		v >>= 8
		for i := 7; i >= 0; i-- {
			ret = append([]byte{byte((v & 0x7f) | 0x80)}, ret...)
			v >>= 7
		}
		return ret
	}

	buf := []byte{}
	for {
		buf = append(buf, byte((v&0x7f)|0x80))
		v >>= 7
		if v == 0 {
			break
		}
	}
	buf[0] &= 0x7f

	ret := make([]byte, len(buf))
	i := 0
	for j := len(buf) - 1; j >= 0; j-- {
		ret[i] = buf[j]
		i++
	}
	return ret
}

/*
// https://sqlite.org/src4/doc/trunk/www/varint.wiki
func decodeVarint(bytes []byte) (uint64, uint) {
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
	}
	ret := uint64(0)
	//fmt.Println(">>>", len(bytes), v-247, v)
	for i := uint64(0); i < v-247; i++ {
		ret = 256*ret + uint64(bytes[i])
	}
	return ret, uint(v - 247)
}

func encodeVarint(v uint64) []byte {
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
*/
