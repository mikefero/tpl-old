package main

import (
	"github.com/mikefero/tpl/db"
)

func main() {
	defer db.Close()
}
