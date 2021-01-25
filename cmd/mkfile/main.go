package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/busoc/prospect"
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
	filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
		now := time.Now()
		if err != nil {
			return err
		}
		if i.IsDir() {
			return nil
		}
		d.File = file

		log.Printf("start processing %s", d.File)
		d, err := processData(d)
		if err != nil {
			log.Printf("fail to update %s: %s", d.File, err)
			return nil
		}
		d = b.GetMime(d)
		if err := b.Store(d); err != nil {
			log.Printf("fail to store %s: %s", d.File, err)
			return nil
		}
		log.Printf("done %s (%d - %s - %s)", d.File, d.Size, time.Since(now), d.MD5)
		return nil
	})
}

func processData(d prospect.Data) (prospect.Data, error) {
	return d, prospect.ReadFile(&d, d.File)
}
