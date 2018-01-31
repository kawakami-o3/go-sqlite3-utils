package sqlite3utils

import (
	"encoding/binary"
	"fmt"

	"io/ioutil"
	"math"
	"os"
	"strconv"
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

func parsePage(cnt []byte, page_num, page_size int) *Page {
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
		fmt.Printf("[%d]WARN: empty page\n", page_num)
		return page // empty
	} else if page.pageType != 13 {
		fmt.Printf("[%d]WARN: Not yet implemented. pageType=%d\n", page_num, page.pageType)
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
	//fmt.Println()
	cellOffset := page.startCellPtr + page_size*(page_num-1)
	//fmt.Printf("Cell Content[Offset=%d]\n", cellOffset)
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

		if cellOffset+payload_size > page_num*page_size {
			fmt.Println("Need to check an overflow page. (exp, act) = ",
				cellOffset+payload_size, page_num*page_size, payload_size)
			return nil
		}

		payload_bytes := fetch(cnt, cellOffset+delta, payload_size)

		v, i = decodeVarint(payload_bytes)
		header_size := int(v)

		header_ints := []uint64{}
		total := int(i)

		dataShift := header_size
		row := &Row{rowid: rowid, datas: []*Data{}}
		for header_size > total {
			//fmt.Println(">", header_size, total)
			v, i = decodeVarint(payload_bytes[total:])
			if i == 0 {
				fmt.Println("internal error")
				return nil
			}
			total += int(i)

			header_ints = append(header_ints, v)

			serialType := int(v)

			d := takeData(payload_bytes[dataShift:], serialType)

			row.datas = append(row.datas, d)
			dataShift += len(d.Bytes)
		}

		page.rows = append(page.rows, row)
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

	// serialTypes []int // Fail in case of "blob" or "text"
	rows []*Row
}

type Row struct {
	rowid uint64
	datas []*Data
}

func takeData(bytes []byte, serialType int) *Data {
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

	bs := bytes[0:size]
	var value string
	if or(serialType, []int{0, 10, 11}) {
		value = ""
	} else if or(serialType, []int{1, 2, 3, 4, 5, 6}) {
		//value = strconv.Itoa(binary.BigEndian.Uint64(bs))
		//value = strconv.FormatUint(binary.BigEndian.Uint64(bs), 10)
		value = strconv.Itoa(toInt(bs))
	} else if serialType == 7 {
		f := math.Float64frombits(binary.BigEndian.Uint64(bs))
		value = strconv.FormatFloat(f, 'e', 8, 64)
	} else if serialType == 8 {
		value = "0"
	} else if serialType == 9 {
		value = "1"
	} else if serialType%2 == 0 {
		value = "[" + strconv.Itoa(int(bs[0]))
		for _, b := range bs[1:] {
			value += "," + strconv.Itoa(int(b))
		}
		value += "]"
	} else {
		value = string(bs)
	}

	return &Data{
		SerialType: serialType,
		Bytes:      bs,
		Value:      value,
	}
}

func or(i int, ns []int) bool {
	for _, n := range ns {
		if i == n {
			return true
		}
	}
	return false
}

type Data struct {
	SerialType int
	Bytes      []byte
	Value      string
}

type Header struct {
	headerString   string
	pageSize       int
	writeVersion   int
	readVersion    int
	reservedSize   int
	payloadMax     int
	payloadMin     int
	payloadLeaf    int
	changeCounter  int
	inHeaderDbSize int

	freeTrunk1st int
	totalFree    int
	schemaCookie int
	schemaNumber int
	cacheSize    int
	logest       int
	encoding     int
	userVersion  int
	vacuumMode   int
	appId        int
	reserved     int
	vvfNum       int
	sqlNum       int
}

func parseHeader(bytes []byte) *Header {
	return &Header{
		headerString:   string(bytes[0:16]),
		pageSize:       fetchInt(bytes, 16, 2),
		writeVersion:   fetchInt(bytes, 18, 1),
		readVersion:    fetchInt(bytes, 19, 1),
		reservedSize:   fetchInt(bytes, 20, 1),
		payloadMax:     fetchInt(bytes, 21, 1),
		payloadMin:     fetchInt(bytes, 22, 1),
		payloadLeaf:    fetchInt(bytes, 23, 1),
		changeCounter:  fetchInt(bytes, 24, 4),
		inHeaderDbSize: fetchInt(bytes, 28, 4),
		freeTrunk1st:   fetchInt(bytes, 32, 4),
		totalFree:      fetchInt(bytes, 36, 4),
		schemaCookie:   fetchInt(bytes, 40, 4),
		schemaNumber:   fetchInt(bytes, 44, 4),
		cacheSize:      fetchInt(bytes, 48, 4),
		logest:         fetchInt(bytes, 52, 4),
		encoding:       fetchInt(bytes, 56, 4),
		userVersion:    fetchInt(bytes, 60, 4),
		vacuumMode:     fetchInt(bytes, 64, 4),
		appId:          fetchInt(bytes, 68, 4),
		reserved:       fetchInt(bytes, 72, 20),
		vvfNum:         fetchInt(bytes, 92, 4),
		sqlNum:         fetchInt(bytes, 96, 4),
	}
}

type Storage struct {
	Path string

	Header *Header
	Pages  []*Page
	Tables map[string]*Table
}

type Entry struct {
	Datas []*Data
}

type Table struct {
	Entries []*Entry
}

func makeTable(rows []*Row) *Table {
	table := &Table{}

	for _, i := range rows {
		table.Entries = append(table.Entries, &Entry{i.datas})
	}

	return table
}

func makeTables(pages []*Page) map[string]*Table {
	m := map[string]*Table{}

	// CREATE TABLE sqlite_master ( type text, name text, tbl_name text, rootpage integer, sql text);
	m["sqlite_master"] = makeTable(pages[0].rows)

	for _, v := range pages[0].rows {
		tableName := v.datas[2].Value
		rootPage, _ := strconv.Atoi(v.datas[3].Value)
		rows := []*Row{}

		for _, r := range pages[rootPage-1].rows {
			rows = append([]*Row{r}, rows...)
		}
		//sort.Sort(sort.Reverse(sort.IntSlice(rows)))
		//m[tableName] = makeTable(pages[rootPage-1].rows)
		m[tableName] = makeTable(rows)
	}

	return m
}

func Load(path string) (*Storage, error) {

	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return nil, err
	}

	cnt, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	header := parseHeader(cnt)

	/*
		fmt.Println()

		// lock-byte  1073741823:1073742336
		if 1073741824 > len(cnt) {
			fmt.Println(1073741824, ">", len(cnt))
		} else {
			fmt.Println(1073741824, "<=", len(cnt))
		}
	*/

	//schemaPage := parsePage(cnt, 1, header.pageSize)
	//pp.Println(schemaPage)

	pages := []*Page{}
	page_no := 0
	for header.pageSize*page_no < len(cnt) {
		page_no++
		page := parsePage(cnt, page_no, header.pageSize)
		//pp.Println(page)
		pages = append(pages, page)
	}

	return &Storage{
		Path:   path,
		Header: header,
		Pages:  pages,
		Tables: makeTables(pages),
	}, nil
}
