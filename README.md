# go-sqlite-utils
[![Go Report Card](https://goreportcard.com/badge/github.com/kawakami-o3/go-sqlite3-utils)](https://goreportcard.com/report/github.com/kawakami-o3/go-sqlite3-utils)
[![Build Status](https://travis-ci.org/kawakami-o3/go-sqlite3-utils.svg?branch=master)](https://travis-ci.org/kawakami-o3/go-sqlite3-utils)

Pure Go Libraries for manipulating a SQLite file.

## Installation

```
go get github.com/kawakami-o3/go-sqlite3-utils
```

## Usage

Load the SQLite file, ```/tmp/test.db```,

```
sqlite3utils.Load("/tmp/test.db")
```

Get the first value at the second row in the table, "person",

```
pages.Tables["person"].Entries[1].Datas[0].Value
```

## Todo

- [x] Complicated file: Now, the parser can read wc.db of subversion.
- [x] Overflow page
- [ ] Schema parser
- [ ] Index page
- [ ] Writer
