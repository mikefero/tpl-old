package main

import (
	"github.com/mikefero/tpl/db"
	"github.com/mikefero/tpl/html"
)

func main() {
	defer db.Close()
	html.ListenAndServe()
}
