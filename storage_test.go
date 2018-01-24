package sqlite3utils

import (
	"fmt"
	"os"
	"os/exec"
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
func TestSimpleLoad(t *testing.T) {

	filename := "/tmp/test.db"
	{
		_, err := os.Stat(filename)
		if err != nil {
			//err := exec.Command("sqlite3", "-cmd 'CREATE TABLE person(id integer, name text);'", filename).Run()
			//err := exec.Command("echo", "'CREATE TABLE person(id integer, name text);' | sqlite3 test.db", filename).Run()
			//err := exec.Command("echo", "'CREATE TABLE person(id integer, name text);' | sqlite3 test.db", filename).Run()
			err := exec.Command("bash", "make_db.sh", filename).Run()
			if err != nil {
				fmt.Println("sqlite3", filename)
			}
		}
	}

	Load(filename)

	{
		_, err := os.Stat(filename)
		if err == nil {
			err := exec.Command("rm", filename).Run()
			if err != nil {
				fmt.Println("rm", filename)
			}
		}
	}
}
