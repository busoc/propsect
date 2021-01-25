package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/busoc/prospect"
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
	filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
		now := time.Now()
		if err != nil || i.IsDir() || !d.Accept(file) {
			return err
		}
		log.Printf("start processing %s", d.File)
		d, err := processData(d, file)
		if err != nil {
			log.Printf("fail to process %s: %s", d.File, err)
			return nil
		}
		ks, err := b.ExecuteCommands(d)
		if err != nil {
			return nil
		}
		d.Links = append(d.Links, ks...)
		if err := b.Store(d); err != nil {
			log.Printf("fail to store %s: %s", d.File, err)
			return nil
		}
		log.Printf("done processing %s (%d - %s - %s)", d.File, d.Size, time.Since(now), d.MD5)
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
