package sqlite3utils

import (
	"encoding/binary"
	"errors"
	"fmt"

	"io/ioutil"
	"math"
	"os"
	"strconv"

	u "github.com/kawakami-o3/undergo"
)

const (
	interiorIndex = 2  // 0x02
	interiorTable = 5  // 0x05
	leafIndex     = 10 // 0x0a
	leafTable     = 13 // 0x0d
)

func warn(msg ...interface{}) {
	fmt.Println(append([]interface{}{"[WARN]"}, msg...)...)
}

func debugPp(msg ...interface{}) {
	//pp.Println(msg)
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

func parseInteriorIndexPage(page *Page, bytes []byte, pageNum, pageSize int) *Page {

	pageOffset := pageSize * (pageNum - 1)
	for _, cellPtr := range page.cellPtrs {
		cellOffset := cellPtr + pageOffset

		var v uint64
		var i uint
		delta := 0
		payloadSize := 0

		childPageNumber := toInt(fetch(bytes, cellOffset, 4))
		delta += 4
		debug("0x02:offset:", u.S16(cellOffset), page.startCellPtr, u.S16(pageSize*(pageNum-1)))
		debug("0x02:child:", childPageNumber, fetch(bytes, cellOffset, 4))

		v, i = decodeVarint32(fetch(bytes, cellOffset+delta, 8))
		delta += int(i)
		payloadSize = int(v)
		//debug("payload size:", payloadSize, i, fetch(bytes, cellOffset, payloadSize+4))
		debug("0x02:payld:", payloadSize, i, fetch(bytes, cellOffset+delta, 8))

		//payloadBytes := fetch(bytes, cellOffset+delta, payloadSize)
		payloadBytes := fetch(bytes, cellOffset+delta, payloadSize)

		v, i = decodeVarint(payloadBytes)
		headerSize := int(v)

		//headerInts := []uint64{}
		total := int(i)

		dataShift := headerSize
		row := &Row{
			childPageNumber: childPageNumber,
			datas:           []*Data{},
		}
		//debug("total vs headerSize", total, headerSize)
		for total < headerSize {
			v, i = decodeVarint(payloadBytes[total:])
			if i == 0 {
				warn("internal error")
				return nil
			}
			total += int(i)

			//headerInts = append(headerInts, v)

			serialType := int(v)

			if len(payloadBytes) < dataShift {
				warn("[", page.pageNum, "]", "dataShift too large", len(payloadBytes), dataShift)
				//debugPp(page)
			}
			d, err := takeData(fetch(payloadBytes, dataShift, 0), serialType)
			if err != nil {
				warn(err)
			}

			row.datas = append(row.datas, d)
			dataShift += len(d.Bytes)
		}

		page.rows = append(page.rows, row)
		//debug("offset:", payloadSize, delta)
		cellOffset += payloadSize + delta
	}

	return page
}

func parseLeafIndexPage(page *Page, bytes []byte, pageNum, pageSize int) *Page {

	pageOffset := pageSize * (pageNum - 1)
	for _, cellPtr := range page.cellPtrs {
		cellOffset := cellPtr + pageOffset

		var v uint64
		var i uint
		delta := 0
		payloadSize := 0

		v, i = decodeVarint32(fetch(bytes, cellOffset, 8))
		delta += int(i)
		payloadSize = int(v)
		//debug("payload size:", payloadSize, i, fetch(bytes, cellOffset, payloadSize+4))
		debug("payld:", payloadSize, i, fetch(bytes, cellOffset, 8))

		//payloadBytes := fetch(bytes, cellOffset+delta, payloadSize)
		payloadBytes := fetch(bytes, cellOffset+delta, payloadSize)

		v, i = decodeVarint(payloadBytes)
		headerSize := int(v)

		//headerInts := []uint64{}
		total := int(i)

		dataShift := headerSize
		row := &Row{
			datas: []*Data{},
		}
		//debug("total vs headerSize", total, headerSize)
		for total < headerSize {
			v, i = decodeVarint(payloadBytes[total:])
			if i == 0 {
				warn("internal error")
				return nil
			}
			total += int(i)

			//headerInts = append(headerInts, v)

			serialType := int(v)

			if len(payloadBytes) < dataShift {
				warn("[", page.pageNum, "]", "dataShift too large", len(payloadBytes), dataShift)
			}
			d, err := takeData(fetch(payloadBytes, dataShift, 0), serialType)
			if err != nil {
				warn(err)
			}

			row.datas = append(row.datas, d)
			dataShift += len(d.Bytes)
		}

		page.rows = append(page.rows, row)
		//debug("offset:", payloadSize, delta)
		cellOffset += payloadSize + delta
	}

	return page
}

func parseInteriorTablePage(page *Page, bytes []byte, pageNum, pageSize int) *Page {
	/*
		Table B-Tree Interior Cell (header 0x05):
			* A 4-byte big-endian page number which is the left child pointer.
			* A varint which is the integer key
	*/
	//pageEnd := pageSize * pageNum

	pageOffset := pageSize * (pageNum - 1)
	for _, cellPtr := range page.cellPtrs {
		cellOffset := cellPtr + pageOffset

		childPageNumber := toInt(fetch(bytes, cellOffset, 4))
		cellOffset += 4
		rowid, n := decodeVarint(fetch(bytes, cellOffset, 8))

		debug("rowid, childPageNumber:", rowid, childPageNumber, fetch(bytes, cellOffset-10, 8+10), cellOffset)
		cellOffset += int(n)

		page.rows = append(page.rows, &Row{
			rowid: rowid,
			//datas: []*Data{},
			childPageNumber: childPageNumber,
		})
	}

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

	pageOffset := pageSize * (pageNum - 1)
	for _, cellPtr := range page.cellPtrs {
		cellOffset := cellPtr + pageOffset

		var v uint64
		var i uint
		delta := 0
		payloadSize := 0

		v, i = decodeVarint32(fetch(bytes, cellOffset, 8))
		delta += int(i)
		payloadSize = int(v)
		//debug("payload size:", payloadSize, i, fetch(bytes, cellOffset, payloadSize+4))
		debug("payld:", payloadSize, i, fetch(bytes, cellOffset, 8))

		v, i = decodeVarint(fetch(bytes, cellOffset+delta, 8))
		rowid := v
		//debug("rowid:", rowid, i, cellOffset, delta, fetch(bytes, cellOffset+delta, 8), len(bytes))
		delta += int(i)

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
		//debug("total vs headerSize", total, headerSize)
		for total < headerSize {
			v, i = decodeVarint(payloadBytes[total:])
			if i == 0 {
				warn("internal error")
				return nil
			}
			total += int(i)

			//headerInts = append(headerInts, v)

			serialType := int(v)

			if len(payloadBytes) < dataShift {
				warn("[", page.pageNum, "]", "dataShift too large", len(payloadBytes), dataShift)
				//debugPp(page)
				//return page // TODO fix
			}
			d, err := takeData(fetch(payloadBytes, dataShift, 0), serialType)
			if err != nil {
				warn(err)
				//return page // TODO fix
			}

			row.datas = append(row.datas, d)
			dataShift += len(d.Bytes)
		}

		page.rows = append(page.rows, row)
		//debug("offset:", payloadSize, delta)
		cellOffset += payloadSize + delta
	}

	return page
}

func parsePage(cnt []byte, pageNum, pageSize int, header *Header) *Page {
	page := &Page{
		pageNum:  pageNum,
		children: make(map[int]*Page),
	}
	//page.pageNum = pageNum

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

	page.freeBlock = toInt(fetch(cnt, offset+1, 2))
	page.cellCount = toInt(fetch(cnt, offset+3, 2))
	page.startCellPtr = toInt(fetch(cnt, offset+5, 2))
	if page.startCellPtr == 0 {
		page.startCellPtr = 65536
	}
	page.fragments = toInt(fetch(cnt, offset+7, 1))

	cellPtrOffset := offset + 8
	if page.pageType == interiorIndex || page.pageType == interiorTable {
		page.rightPtr = toInt(fetch(cnt, offset+8, 4))
		cellPtrOffset = offset + 12
	}

	// empty page
	if page.pageType == 0 {
		//warn("empty page\n", pageNum)
		return page
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

	page.cellPtrs = []int{}
	for i := 0; i < page.cellCount; i++ {
		page.cellPtrs = append(page.cellPtrs, toInt(fetch(cnt, cellPtrOffset+2*i, 2)))
	}

	// bytes: content witout free blocks
	bytes := cnt

	if page.pageType == interiorTable {
		parseInteriorTablePage(page, bytes, pageNum, pageSize)
	} else if page.pageType == leafTable {
		parseLeafTablePage(page, bytes, pageNum, pageSize)
	} else if page.pageType == interiorIndex {
		parseInteriorIndexPage(page, bytes, pageNum, pageSize)
	} else if page.pageType == leafIndex {
		parseLeafIndexPage(page, bytes, pageNum, pageSize)
	}

	//debugPp(page)
	return page
}

// Page ...
type Page struct {
	pageNum      int
	pageType     int
	freeBlock    int
	cellCount    int
	startCellPtr int
	fragments    int
	rightPtr     int
	children     map[int]*Page

	cellPtrs []int

	// serialTypes []int // Fail in case of "blob" or "text"
	rows []*Row
}

func (page *Page) selectFirstChild(pages []*Page) *Page {
	number := page.rows[0].childPageNumber
	if number <= 0 {
		number = page.rightPtr
	}
	if number <= 0 {
		panic("selectFirstChild: no child")
	}
	return pages[number-1]
}

// Row ...
type Row struct {
	rowid uint64

	datas []*Data // in a leaf table

	childPageNumber int // 4-byte integer in an interior table
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
		return nil, fmt.Errorf("No enough bytes! [%d < %d]", len(bytes), size)
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
		//debug("blob: type, len = ", serialType, len(bs))
		value = "["
		for i, b := range bs {
			if i > 0 {
				value += ","
			}
			value += strconv.Itoa(int(b))
			if i > 8 {
				value += "..."
				break
			}
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

	usableSize int
	maxLocal   int
	minLocal   int
	maxLeaf    int
	minLeaf    int
}

func parseHeader(bytes []byte) *Header {
	header := &Header{
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

	usableSize := header.pageSize - header.reservedSize
	header.usableSize = usableSize
	header.maxLocal = (usableSize-12)*64/255 - 23
	header.minLocal = (usableSize-12)*32/255 - 23
	header.maxLeaf = usableSize - 35
	header.minLeaf = (usableSize-12)*32/255 - 23

	return header
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

func makeTable(pages []*Page) *Table {
	table := &Table{}

	for _, p := range pages {
		for _, i := range p.rows {
			table.Entries = append(table.Entries, &Entry{i.datas})
		}
	}

	return table
}

func fillChildren(pages []*Page) {
	for _, page := range pages {
		if page.rightPtr != 0 {
			page.children[page.rightPtr] = pages[page.rightPtr-1]
		}

		if page.pageType == interiorTable {
			for _, r := range page.rows {
				number := r.childPageNumber
				page.children[number] = pages[number-1]
			}
		}
	}
}

func selectFirstLeafTable(pages ...*Page) *Page {
	page := pages[0]
	for page.pageType != leafTable {
		page = page.selectFirstChild(pages)
	}
	return page
}

func walkPage(page *Page, pageType int) []*Page {
	ret := []*Page{}
	if page.pageType == pageType {
		ret = append(ret, page)
	}
	for _, p := range page.children {
		ret = append(ret, walkPage(p, pageType)...)
	}
	return ret
}

func makeTables(pages []*Page) (map[string]*Table, error) {
	m := map[string]*Table{}

	// CREATE TABLE sqlite_master ( type text, name text, tbl_name text, rootpage integer, sql text);

	masterPages := []*Page{}

	firstPageType := pages[0].pageType
	if firstPageType == leafTable {
		masterPages = append(masterPages, pages[0])
	} else if firstPageType == interiorTable {
		for _, page := range pages[0].children {
			masterPages = append(masterPages, page)
		}
	} else {
		panic("!!!")
	}

	m["sqlite_master"] = makeTable(masterPages)
	//pp.Println(m["sqlite_master"])

	for _, v := range m["sqlite_master"].Entries {
		tableName := v.Datas[2].Value
		rootPageNum, err := strconv.Atoi(v.Datas[3].Value)
		if err != nil {
			return nil, err
		}

		if rootPageNum == 0 {
			continue
		}
		rootPage := pages[rootPageNum-1]
		if rootPage.pageType == 2 || rootPage.pageType == 10 {
			continue
		}

		m[tableName] = makeTable(walkPage(rootPage, leafTable))
	}

	return m, nil
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
		// lock-byte  1073741823:1073742336
		if 1073741824 > len(cnt) {
			fmt.Println(1073741824, ">", len(cnt))
		} else {
			fmt.Println(1073741824, "<=", len(cnt))
		}
	*/

	//schemaPage := parsePage(cnt, 1, header.pageSize)

	pages := []*Page{}
	pageNo := 0
	freeCount := 0
	for header.pageSize*pageNo < len(cnt) {
		pageNo++
		page := parsePage(cnt, pageNo, header.pageSize, header)
		pages = append(pages, page)

		if page.pageType == 0 {
			freeCount++
			//pp.Println(page)
		}

	}

	fillChildren(pages)

	tables, err := makeTables(pages)
	if err != nil {
		return nil, err
	}

	return &Storage{
		Path:   path,
		Header: header,
		Pages:  pages,
		Tables: tables,
	}, nil
}
