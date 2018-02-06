package sqlite3utils

import (
	"encoding/binary"
	"errors"
	"fmt"

	"io/ioutil"
	"math"
	"os"
	"strconv"
)

func warn(msg ...interface{}) {
	fmt.Println(append([]interface{}{"[WARN]"}, msg...)...)
}

func debug(msg ...interface{}) {
	//fmt.Println(msg...)
}

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
	}
	return bytes[offset : offset+size]
}

func fetchInt(bytes []byte, offset, size int) int {
	return toInt(bytes[offset : offset+size])
}

func parseIndexInteriorTablePage(page *Page, bytes []byte, pageNum, pageSize int) *Page {
	debug("index interiror")
	return page
}
func parseIndexLeafTablePage(page *Page, bytes []byte, pageNum, pageSize int) *Page {
	debug("index leaf")
	return page
}

func parseInteriorTablePage(page *Page, bytes []byte, pageNum, pageSize int) *Page {
	/*
		Table B-Tree Interior Cell (header 0x05):
			* A 4-byte big-endian page number which is the left child pointer.
			* A varint which is the integer key
	*/
	cellOffset := page.startCellPtr + pageSize*(pageNum-1)
	pageEnd := pageSize * pageNum

	for row := 0; row < page.cellCount; row++ {
		debug("...", row)
		childPageNumber := toInt(fetch(bytes, cellOffset, 4))
		cellOffset += 4
		rowid, n := decodeVarint(fetch(bytes, cellOffset, 0))
		cellOffset += int(n)

		page.rows = append(page.rows, &Row{
			rowid: rowid,
			//datas: []*Data{},
			childPageNumber: childPageNumber,
		})
	}

	debug(bytes[cellOffset:pageEnd])

	return page
}

func parseLeafTablePage(page *Page, bytes []byte, pageNum, pageSize int) *Page {
	// In case of type=13 ...
	/*
		Table B-Tree Leaf Cell (header 0x0d):
		* A varint which is the total number of bytes of payload, including any overflow
		* A varint which is the integer key, a.k.a. "rowid"
		* The initial portion of the payload that does not spill to overflow pages.
		* A 4-byte big-endian integer page number for the first page of
			the overflow page list - omitted if all payload fits on the b-tree page.
	*/
	cellOffset := page.startCellPtr + pageSize*(pageNum-1)

	for row := 0; row < page.cellCount; row++ {

		var v uint64
		var i uint
		delta := 0
		payloadSize := 0

		v, i = decodeVarint(fetch(bytes, cellOffset, 8))
		delta += int(i)
		payloadSize = int(v)
		debug("payload size:", payloadSize, i, fetch(bytes, cellOffset, payloadSize+4))

		v, i = decodeVarint(fetch(bytes, cellOffset+delta, 8))
		delta += int(i)
		rowid := v

		debug("rowId:", rowid, i)
		if cellOffset+delta+payloadSize > pageNum*pageSize {
			warn("Need to check an overflow page. (exp, act) = ",
				cellOffset+payloadSize, pageNum*pageSize, payloadSize)
			return nil
		}

		//payloadBytes := fetch(bytes, cellOffset+delta, payloadSize)
		payloadBytes := fetch(bytes, cellOffset+delta, payloadSize)

		v, i = decodeVarint(payloadBytes)
		headerSize := int(v)

		//headerInts := []uint64{}
		total := int(i)

		dataShift := headerSize
		row := &Row{rowid: rowid, datas: []*Data{}}
		debug("total vs headerSize", total, headerSize)
		for total < headerSize {
			v, i = decodeVarint(payloadBytes[total:])
			if i == 0 {
				warn("internal error")
				return nil
			}
			total += int(i)

			//headerInts = append(headerInts, v)

			serialType := int(v)

			debug(len(payloadBytes), dataShift)
			if len(payloadBytes) < dataShift {
				warn("dataShift too large", len(payloadBytes), dataShift)
				debug(pageNum, pageSize)
				return page // TODO fix
			}
			d, err := takeData(fetch(payloadBytes, dataShift, 0), serialType)
			if err != nil {
				warn(err)
				return page // TODO fix
			}

			//pp.Println(d)
			row.datas = append(row.datas, d)
			dataShift += len(d.Bytes)
		}

		page.rows = append(page.rows, row)
		//pp.Print(row)
		debug("offset:", payloadSize, delta)
		cellOffset += payloadSize + delta
	}

	return page
}

func parsePage(cnt []byte, pageNum, pageSize int) *Page {
	page := &Page{}

	offset := pageSize * (pageNum - 1)
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
		fmt.Printf("[%d]WARN: empty page\n", pageNum)
		return page // empty
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

	page.cellPtrOffset = toInt(fetch(cnt, cellPtrOffset, 2))
	/*
		if page.cellPtrOffset > page.freeBlock && page.freeBlock != 0 {
			fmt.Printf("[%d]WARN: free blocks before cells\n", pageNum)
		}
	*/

	/*
		// bytes: content witout free blocks
		freeBlockPtr := offset + page.freeBlock
		bytes := cnt[0:freeBlockPtr]
		//pp.Println(page)
		for 0 < freeBlockPtr-offset {
			// free block
			//  | 1   | 2    | 3          | 4              | ...     |
			//  | next block | block size including header | empty   |

			nextFreeBlockPtr := offset + toInt(fetch(cnt, freeBlockPtr, 2))
			freeBlockSize := toInt(fetch(cnt, freeBlockPtr+2, 2))

			if nextFreeBlockPtr == offset {
				bytes = append(bytes, cnt[freeBlockPtr+freeBlockSize:offset+pageSize]...)
			} else {
				bytes = append(bytes, cnt[freeBlockPtr+freeBlockSize:nextFreeBlockPtr]...)
			}

			freeBlockPtr = nextFreeBlockPtr
		}
	*/

	page.printHeader()
	//pageEnd = pageNum * pageSize
	//debug(bytes[pageEnd-ZZ
	if page.freeBlock > 0 {
		return page // TODO fix: avoid the crash for the page including a free block
	}

	if page.pageType == 5 {
		parseInteriorTablePage(page, cnt, pageNum, pageSize)
	} else if page.pageType == 13 {
		parseLeafTablePage(page, cnt, pageNum, pageSize)
	} else if page.pageType == 2 {
		parseIndexInteriorTablePage(page, cnt, pageNum, pageSize)
	} else if page.pageType == 10 {
		parseIndexLeafTablePage(page, cnt, pageNum, pageSize)
	}
	return page
}

// Page ...
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

func (page *Page) printHeader() {
	debug("==================================")
	debug("pageType:", page.pageType)
	debug("freeBlock:", page.freeBlock)
	debug("cellCount:", page.cellCount)
	debug("CellPtr:", page.startCellPtr)
	debug("fragment:", page.fragmentBytes)
	debug("rightPtr:", page.rightPtr)
	debug("cellOffset:", page.cellPtrOffset)
	debug("==================================")
}

// Row ...
type Row struct {
	rowid uint64

	datas           []*Data // in a leaf table
	childPageNumber int     // 4-byte integer in an interior table
}

func takeData(bytes []byte, serialType int) (*Data, error) {
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
	} else if serialType%2 == 1 {
		size = (serialType - 13) / 2
	} else {
		return nil, errors.New("Unkown serialType")
	}

	if len(bytes) < size {
		return nil, errors.New(fmt.Sprintf("no enough bytes! [%d < %d]\n", len(bytes), size))
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
		debug(serialType, bs)
		value = "["
		for i, b := range bs {
			if i > 0 {
				value += ","
			}
			value += strconv.Itoa(int(b))
		}
		value += "]"
	} else {
		value = string(bs)
	}

	return &Data{
		SerialType: serialType,
		Bytes:      bs,
		Value:      value,
		Len:        len(bs),
	}, nil
}

func or(i int, ns []int) bool {
	for _, n := range ns {
		if i == n {
			return true
		}
	}
	return false
}

// Data ...
type Data struct {
	SerialType int
	Bytes      []byte
	Value      string
	Len        int
}

// Header ...
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
	appID        int
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
		appID:          fetchInt(bytes, 68, 4),
		reserved:       fetchInt(bytes, 72, 20),
		vvfNum:         fetchInt(bytes, 92, 4),
		sqlNum:         fetchInt(bytes, 96, 4),
	}
}

// Storage ...
type Storage struct {
	Path string

	Header *Header
	Pages  []*Page
	Tables map[string]*Table
}

// Entry ...
type Entry struct {
	Datas []*Data
}

// Table ...
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
	return m // TODO fix

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

// Load ...
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
	pageNo := 0
	for header.pageSize*pageNo < len(cnt) {
		pageNo++
		page := parsePage(cnt, pageNo, header.pageSize)
		//pp.Println(page)
		pages = append(pages, page)

		//break // TODO delete
	}

	return &Storage{
		Path:   path,
		Header: header,
		Pages:  pages,
		Tables: makeTables(pages),
	}, nil
}
