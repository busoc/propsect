package main

import (
	"errors"

	"github.com/midbel/cli"
)

var (
	ErrMismatched = errors.New("checksum mismatched")
	ErrSize       = errors.New("size mismatched")
)

const help = `{.Name} copies files throught SFTP to a remote server

Usage:
  {{.Name}} command [arguments]

The commands are:

{{range .Commands}}{{printf "  %-9s %s" .String .Short}}
{{end}}

Use {{.Name}} [command] -h for more information about its usage.
`

func main() {
	commands := []*cli.Command{
		{
			Usage: "copy [-q] <config>",
			Short: "transfer files via sftp to a remote server",
			Alias: []string{"transfer"},
			Run:   runCopy,
		},
		{
			Usage: "monitor <addr>",
			Short: "",
			Run:   runMonitor,
		},
	}
	cli.RunAndExit(commands, cli.Usage("secop", help, commands))
}
