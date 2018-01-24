#!/bin/sh



echo 'CREATE TABLE person(id integer, name text);' | sqlite3 $@
#echo $@


