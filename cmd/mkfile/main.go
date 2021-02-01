package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/busoc/prospect"
	"github.com/busoc/prospect/cmd/internal/trace"
)

func main() {
	flag.Parse()

	err := prospect.Build(flag.Arg(0), collectData, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func collectData(b prospect.Builder, d prospect.Data) {
	tracer := trace.New("mkfile")
	defer tracer.Summarize()
	filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
		if err != nil || i.IsDir() {
			return err
		}
		dat := d.Clone()
		dat.File = file

		tracer.Start(file)
		defer tracer.Done(file, dat)
		if dat, err = processData(dat); err != nil {
			tracer.Error(file, err)
			return nil
		}
		dat = b.GetMime(dat)
		if err := b.Store(dat); err != nil {
			tracer.Error(file, err)
		}
		return nil
	})
}

func processData(d prospect.Data) (prospect.Data, error) {
	return d, prospect.ReadFile(&d, d.File)
}
