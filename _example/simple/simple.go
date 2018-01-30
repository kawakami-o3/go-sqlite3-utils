package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	sqlite3utils "github.com/kawakami-o3/go-sqlite3-utils"
)

func execSQLite(filename string, queries []string) {
	script, _ := filepath.Abs("../../script/sqlite.rb")
	escape := regexp.MustCompile(`'`)
	for _, q := range queries {
		q = escape.ReplaceAllString(q, "\\'")
		//fmt.Print("Query> " + q)
		//out, err := exec.Command("ruby", script, filename, q).Output()
		err := exec.Command("ruby", script, filename, q).Run()
		//fmt.Print("Result> " + string(out))
		if err != nil {
			panic(err)
		}
	}
}

func rmSQLite(filename string) {
	_, err := os.Stat(filename)
	if err == nil {
		err := exec.Command("rm", filename).Run()
		if err != nil {
			panic(err)
		}
	}
}

func main() {
	filename := "/tmp/test.db"
	rmSQLite(filename)

	execSQLite(filename, []string{
		"CREATE TABLE person(id integer, name text);",
		"INSERT INTO person VALUES (1, \"hoge\");",
		"INSERT INTO person VALUES (2, \"foo\");",
		"INSERT INTO person VALUES (3, \"bar\");",
	})

	pages, _ := sqlite3utils.Load(filename)
	fmt.Print(pages.Tables["person"].Entries[0].Datas[0].Value)
	fmt.Print("|")
	fmt.Print(pages.Tables["person"].Entries[0].Datas[1].Value)
	fmt.Println()
	fmt.Print(pages.Tables["person"].Entries[1].Datas[0].Value)
	fmt.Print("|")
	fmt.Print(pages.Tables["person"].Entries[1].Datas[1].Value)
	fmt.Println()
	fmt.Print(pages.Tables["person"].Entries[2].Datas[0].Value)
	fmt.Print("|")
	fmt.Print(pages.Tables["person"].Entries[2].Datas[1].Value)
	fmt.Println()

	rmSQLite(filename)
}
