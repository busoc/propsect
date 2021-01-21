package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/busoc/prospect/builder"
)

const help = `create an archive from experiment data files

options:

  -h       show this help message and exit
  -s FILE  use FILE as schedule

usage: prospect [-s schedule.csv] prospect.toml
`

func main() {
	flag.Usage = func() {
		fmt.Println(strings.TrimSpace(help))
		os.Exit(2)
	}
	schedule := flag.String("s", "", "schedule")
	flag.Parse()
	b, err := builder.New(flag.Arg(0), *schedule)
	if err != nil {
		fmt.Fprintln(os.Stderr, "configure:", err)
		os.Exit(1)
	}
	defer b.Close()
	if err := b.Build(); err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
}
