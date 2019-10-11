package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/busoc/prospect"
)

func main() {
	flag.Parse()
	b, err := prospect.NewBuilder(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "configure:", err)
		os.Exit(1)
	}
	defer b.Close()
	if err := b.Build(); err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(2)
	}
}
