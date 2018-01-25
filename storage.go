package sqlite3utils

import (
	"fmt"

	"io/ioutil"
	"math"
	"os"
	"strconv"

	"github.com/k0kubun/pp"
)

/***********************************************************

schema_master
CREATE TABLE sqlite_master(
	type text,
	name text,
	tbl_name text,
	rootpage integer,
	sql text
);

type =>  'table', 'index', 'view', or 'trigger'

***********************************************************/

const (
	ColNull = iota
	ColInt8
	ColInt16
	ColInt24
	ColInt32
	ColInt48
	ColInt64
	ColFloat64
	ColJust0
	ColJust1
	ColBlob
	ColText
	ColReserved
)

func parseSerialType(typeId int) {
}

func toInt(bytes []byte) int {
	l := len(bytes)
	ret := 0
	for i, b := range bytes {
		ret += int(math.Pow(2, float64(8*(l-i-1)))) * int(b)
		//ret += int(math.Pow(2, float64(8*i))) * int(b)
	}
	return ret
}

func fetch(bytes []byte, offset, size int) []byte {
	if size == 0 {
		return bytes[offset:]
	} else {
		return bytes[offset : offset+size]
	}
}

func fetchInt(bytes []byte, offset, size int) int {
	return toInt(bytes[offset : offset+size])
}

/*
func printPageHeader(page map[string]int) {
	fmt.Println("type :", page["page_type"])
	fmt.Println("free :", page["free_block"])
	fmt.Println("cell :", page["cell_number"])
	fmt.Println("start:", page["start_cell"])
	fmt.Println("frags:", page["fragment_bytes"])
}
*/

func procPage(cnt []byte, page_num, page_size int) *Page {
	page := &Page{}

	offset := page_size * (page_num - 1)
	if offset == 0 {
		offset = 100 // database header in the first page
	}
	/*
		2 : interior index b-tree page
		5 : interior table b-tree page
		10: leaf index b-tree page
		13: leaf table b-tree page
	*/
	page.pageType = toInt(fetch(cnt, offset, 1))
	if page.pageType == 0 {
		fmt.Println("DEBUG: EMPTY page :", page_num)
		l := 30
		fmt.Println(offset)
		fmt.Println(fetch(cnt, offset-l/2, l))
		return page // empty
	} else if page.pageType != 13 {
		fmt.Println("DEBUG: Not yet implemented.")
		return page
	}
	page.freeBlock = toInt(fetch(cnt, offset+1, 2))
	page.cellCount = toInt(fetch(cnt, offset+3, 2))
	page.startCellPtr = toInt(fetch(cnt, offset+5, 2))
	if page.startCellPtr == 0 {
		page.startCellPtr = 65536
	}
	page.fragmentBytes = toInt(fetch(cnt, offset+7, 1))

	cellPtrOffset := 8
	if page.pageType == 5 {
		page.rightPtr = toInt(fetch(cnt, offset+8, 4))
		cellPtrOffset = 12
	}
	/*
		A b-tree page is divided into regions in the following order:

			1. The 100-byte database file header (found on page 1 only)
			2. The 8 or 12 byte b-tree page header
			3. The cell pointer array
			4. Unallocated space
			5. The cell content area
			6. The reserved region.
	*/
	/*
		free block
			| 1   | 2    | 3          | 4              | ...     |
			| next block | block size including header | empty   |
	*/

	page.cellPtrOffset = toInt(fetch(cnt, cellPtrOffset, 2))

	// In case of type=13 ...
	fmt.Println()
	cellOffset := page.startCellPtr + page_size*(page_num-1)
	fmt.Printf("Cell Content[Offset=%d]\n", cellOffset)
	//fmt.Println(fetch(cnt, cellOffset, page_size+offset-cellOffset+10))

	//rest_size := page_size - page["start_cell"]

	//row_count := 0
	for row := 0; row < page.cellCount; row++ {

		var v uint64
		var i uint
		delta := 0
		payload_size := 0

		v, i = decodeVarint(fetch(cnt, cellOffset, 8))
		delta += int(i)
		payload_size = int(v)

		v, i = decodeVarint(fetch(cnt, cellOffset+delta, 8))
		delta += int(i)
		rowid := v
		fmt.Println("rowid:", rowid, i)

		if cellOffset+payload_size > page_num*page_size {
			fmt.Println("Need to check an overflow page. (exp, act) = ",
				cellOffset+payload_size, page_num*page_size, payload_size)
			return nil
		}

		payload_bytes := fetch(cnt, cellOffset+delta, payload_size)

		v, i = decodeVarint(payload_bytes)
		header_size := int(v)

		header_ints := []uint64{}
		column_desc := []string{}
		column_size := []int{}
		total := int(i)
		for header_size > total {
			fmt.Println(">", header_size, total)
			v, i = decodeVarint(payload_bytes[total:])
			if i == 0 {
				fmt.Println("internal error")
				return nil
			}
			total += int(i)

			header_ints = append(header_ints, v)

			serial_type := int(v)
			page.serialType = append(page.serialType, serial_type)

			var desc string
			var size int
			if serial_type == 0 {
				desc = "null"
				size = 0
			} else if serial_type == 1 {
				desc = "int"
				size = 1
			} else if serial_type == 2 {
				desc = "int"
				size = 2
			} else if serial_type == 3 {
				desc = "int"
				size = 3
			} else if serial_type == 4 {
				desc = "int"
				size = 4
			} else if serial_type == 5 {
				desc = "int"
				size = 6
			} else if serial_type == 6 {
				desc = "int"
				size = 8
			} else if serial_type == 7 {
				desc = "float64"
				size = 8
			} else if serial_type == 8 {
				desc = "just0"
				size = 0
			} else if serial_type == 9 {
				desc = "just1"
				size = 0
			} else if serial_type == 10 {
				desc = "not_used"
				size = 0
			} else if serial_type == 11 {
				desc = "not_used"
				size = 0
			} else if serial_type%2 == 0 {
				desc = "blob"
				size = (serial_type - 12) / 2
			} else { // odd
				//desc = fmt.Sprintf("text[%d]", (serial_type-13)/2)
				desc = "text"
				size = (serial_type - 13) / 2
			}

			column_desc = append(column_desc, desc)
			column_size = append(column_size, size)
		}
		fmt.Println("headers:", header_ints)
		fmt.Println("column_desc:", column_desc)
		fmt.Println("column_size:", column_size)

		column_shift := 0
		for i, s := range column_size {
			if column_desc[i] == "text" {
				fmt.Println(string(fetch(payload_bytes, header_size+column_shift, s)))
			} else if column_desc[i] == "int" {
				fmt.Println(toInt(fetch(payload_bytes, header_size+column_shift, s)))
			} else {
				fmt.Println(fetch(payload_bytes, header_size+column_shift, s))
			}
			column_shift += s
		}

		cellOffset += payload_size + delta
	}

	return page
}

type Page struct {
	pageType      int
	freeBlock     int
	cellCount     int
	startCellPtr  int
	fragmentBytes int
	rightPtr      int
	cellPtrOffset int
	child         *Page

	serialType []int
	datas      [][]byte
}

/*
const (
	SerialTypeNull = 0
	SerialTypeInt8 = 1
	SerialTypeInt16 = 2
	SerialTypeInt24 = 3
	SerialTypeInt32 = 4
	SerialTypeInt48 = 5
	SerialTypeInt64 = 6
	SerialTypeFloat64 = 7
	SerialTypeJust0 = 8
	SerialTypeJust1 = 9
	SerialTypeNotUsed0 = 10
	SerialTypeNotUsed1 = 11
	// SerialTypeBlob
	// SerialTypeText
)
*/

func takeData(bytes []byte, serialType int) []byte {
	var size int
	if serialType == 0 {
		size = 0
	} else if serialType == 1 {
		size = 1
	} else if serialType == 2 {
		size = 2
	} else if serialType == 3 {
		size = 3
	} else if serialType == 4 {
		size = 4
	} else if serialType == 5 {
		size = 6
	} else if serialType == 6 {
		size = 8
	} else if serialType == 7 {
		size = 8
	} else if serialType == 8 {
		size = 0
	} else if serialType == 9 {
		size = 0
	} else if serialType == 10 {
		size = 0
	} else if serialType == 11 {
		size = 0
	} else if serialType%2 == 0 {
		size = (serialType - 12) / 2
	} else { // odd
		size = (serialType - 13) / 2
	}

	return bytes[0:size]
}

type Data struct {
	dataType int
	bytes    []byte
	value    string
}

const (
	DataTypeNil = iota + 1
	DataTypeInt
	DataTypeInt64
	DataTypeFloat64
	DataTypeBool
	DataTypeBytes
	DataTypeString
	DataTypeTime
)

type Storage struct {
	filepath string
	bytes    []byte
	pos      uint
}

func LoadSqlite(path string) (*Storage, error) {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return nil, err
	}

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	fmt.Println(bytes)

	//return &Storage{path, bytes, 0}, nil
	return &Storage{}, nil
}

func (s *Storage) take(n uint) []byte {
	i := s.pos
	s.pos += n
	return s.bytes[i : i+n]
}

type Header struct {
	headerString   string
	pageSize       uint
	writeVersion   uint
	readVersion    uint
	reservedSize   uint
	payloadMax     uint
	payloadMin     uint
	payloadLeaf    uint
	changeCounter  uint
	inHeaderDbSize uint

	freeTrunk1st uint
	totalFree    uint
	schemaCookie uint
	schemaNumber uint
	cacheSize    uint
	logest       uint
	encoding     uint
	userVersion  uint
	vacuumMode   uint
	appId        uint
	reserved     uint
	vvfNum       uint
	sqlNum       uint
}

func Load(path string) {

	file, err := os.Open(path)
	//file, err := os.Open("test.db")
	//file, err := os.Open("wc.db")
	//file, err := os.Open("root.wc.db")
	defer file.Close()
	if err != nil {
		panic(err)
	}

	cnt, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}

	header := map[string]string{}
	header["header_string"] = string(cnt[0:16])
	header["page_size"] = strconv.Itoa(fetchInt(cnt, 16, 2))
	header["write_version"] = strconv.Itoa(fetchInt(cnt, 18, 1))
	header["read_version"] = strconv.Itoa(fetchInt(cnt, 19, 1))
	header["reserved_size"] = strconv.Itoa(fetchInt(cnt, 20, 1))
	header["payload_max"] = strconv.Itoa(fetchInt(cnt, 21, 1))
	header["payload_min"] = strconv.Itoa(fetchInt(cnt, 22, 1))
	header["payload_leaf"] = strconv.Itoa(fetchInt(cnt, 23, 1))
	header["change_counter"] = strconv.Itoa(fetchInt(cnt, 24, 4))
	header["in-hdr_db_size"] = strconv.Itoa(fetchInt(cnt, 28, 4))
	header["1st_free_trunk"] = strconv.Itoa(fetchInt(cnt, 32, 4))
	header["total_free"] = strconv.Itoa(fetchInt(cnt, 36, 4))
	header["schema_cookie"] = strconv.Itoa(fetchInt(cnt, 40, 4))
	header["schema_number"] = strconv.Itoa(fetchInt(cnt, 44, 4))
	header["cache_size"] = strconv.Itoa(fetchInt(cnt, 48, 4))
	header["logest"] = strconv.Itoa(fetchInt(cnt, 52, 4))
	header["encoding"] = strconv.Itoa(fetchInt(cnt, 56, 4))
	header["user_version"] = strconv.Itoa(fetchInt(cnt, 60, 4))
	header["vacuum_mode"] = strconv.Itoa(fetchInt(cnt, 64, 4))
	header["app_id"] = strconv.Itoa(fetchInt(cnt, 68, 4))
	header["reserved"] = strconv.Itoa(fetchInt(cnt, 72, 20))
	header["vvf_num"] = strconv.Itoa(fetchInt(cnt, 92, 4))
	header["sql_num"] = strconv.Itoa(fetchInt(cnt, 96, 4))

	for _, i := range []string{
		"header_string",
		"page_size",
		"write_version",
		"read_version",
		"reserved_size",
		"payload_max",
		"payload_min",
		"payload_leaf",
		"change_counter",
		"in-hdr_db_size",

		"1st_free_trunk",
		"total_free",
		"schema_cookie",
		"schema_number",
		"cache_size",
		"logest",
		"encoding",
		"user_version",
		"vacuum_mode",
		"app_id",
		"reserved",
		"vvf_num",
		"sql_num",
	} {
		fmt.Printf(" %-14s : %s\n", i, header[i])
	}

	/*
		fmt.Println()

		// lock-byte  1073741823:1073742336
		if 1073741824 > len(cnt) {
			fmt.Println(1073741824, ">", len(cnt))
		} else {
			fmt.Println(1073741824, "<=", len(cnt))
		}
	*/

	page_size, _ := strconv.Atoi(header["page_size"])

	//load_size := 100
	schema_page := procPage(cnt, 1, page_size)
	pp.Println(schema_page)

	page_no := 1
	for 100+page_size*page_no < len(cnt) {
		page_no++
		fmt.Println("-------------------------------")
		page := procPage(cnt, page_no, page_size)
		pp.Println(page)
	}

}
