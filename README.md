# go-sqlite-utils
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

Get the first value at the first row in the table, "person",

```
pages.Tables["person"].Entries[0].Datas[0].Value
```

## Todo

* Complicated file (v0.0 can parse a very simple case only)
* Overflow page
* Schema parser
* Index page
* Writer
