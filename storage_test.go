package sqlite3utils

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

/*
func assertEq(t *testing.T, expect, actual byte) {
	if expect != actual {
		t.Error("Expected:", expect, "Actual:", actual)
	}
}

func assertEqAll(t *testing.T, expect byte, actual []byte) {

	for i, a := range actual {
		if expect != a {
			t.Error("Expected", expect, "Actual", a, "at", i, "in", actual)
		}
	}

}
*/

func execSQLite(filename string, queries []string) {
	script, _ := filepath.Abs("./script/sqlite.rb")
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

func TestSimpleLoad(t *testing.T) {
	filename := "/tmp/test.db"
	rmSQLite(filename)

	execSQLite(filename, []string{
		"CREATE TABLE person(id integer, name text);",
		"INSERT INTO person VALUES (1, \"hoge\");",
		"INSERT INTO person VALUES (2, \"foo\");",
		"INSERT INTO person VALUES (3, \"bar\");",
	})

	Load(filename)

	//rmSQLite(filename)
}
