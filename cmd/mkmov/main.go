package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/busoc/prospect"
	"github.com/busoc/prospect/cmd/internal/trace"
	"github.com/midbel/exif/mov"
)

const Mime = "video/quicktime"

func main() {
	flag.Parse()

	accept := func(d prospect.Data) bool {
		return d.Mime == Mime
	}
	err := prospect.Build(flag.Arg(0), collectData, accept)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func collectData(b prospect.Builder, d prospect.Data) {
	tracer := trace.New("mkmov")
	defer tracer.Summarize()
	filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
		if err != nil || i.IsDir() || !d.Accept(file) {
			return err
		}
		tracer.Start(file)

		dat := d.Clone()
		if dat, err = processData(dat, file); err != nil {
			tracer.Error(file, err)
			return nil
		}
		ks, err := b.ExecuteCommands(dat)
		if err != nil {
			tracer.Error(file, err)
			return nil
		}
		dat.Links = append(dat.Links, ks...)
		if err := b.Store(dat); err != nil {
			tracer.Error(file, err)
			return nil
		}
		tracer.Done(file, dat)
		return nil
	})
}

func processData(d prospect.Data, file string) (prospect.Data, error) {
	if err := prospect.ReadFile(&d, file); err != nil {
		return d, err
	}
	if err := extractMeta(&d); err != nil {
		return d, err
	}
	return d, nil
}

func extractMeta(d *prospect.Data) error {
	qt, err := mov.Decode(d.File)
	if err != nil {
		return err
	}
	defer qt.Close()

	p, err := qt.DecodeProfile()
	if err != nil {
		return err
	}
	d.AcqTime = p.AcqTime()
	d.ModTime = p.ModTime()

	a := prospect.MakeParameter(prospect.FileDuration, p.Length())
	d.Parameters = append(d.Parameters, a)

	return nil
}
