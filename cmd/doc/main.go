package main

import (
	"os"

	"github.com/segiddins/chrb"
	docs "github.com/urfave/cli-docs/v3"
)

func main() {
	app := chrb.App(nil)

	md, err := docs.ToMarkdown(app)
	if err != nil {
		panic(err)
	}

	fi, err := os.Create("cli-docs.md")
	if err != nil {
		panic(err)
	}
	defer fi.Close()
	if _, err := fi.WriteString("# CLI\n\n" + md); err != nil {
		panic(err)
	}
}
