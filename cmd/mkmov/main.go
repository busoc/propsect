package main

import (
	"crypto/md5"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/busoc/prospect"
	"github.com/midbel/exif/mov"
	"github.com/midbel/toml"
)

const Mime = "video/quicktime"

type Data struct {
	Archive string
	prospect.Data
}

func (d Data) Process(file string) (Data, error) {
	d.File = file
	r, err := os.Open(d.File)
	if err != nil {
		return d, err
	}
	defer r.Close()

	var (
		sumSHA = sha256.New()
		sumMD5 = md5.New()
	)
	if d.Size, err = io.Copy(io.MultiWriter(sumSHA, sumMD5), r); err != nil {
		return d, err
	}

	d.Integrity = prospect.SHA
	d.Sum = fmt.Sprintf("%x", sumSHA.Sum(nil))
	d.MD5 = fmt.Sprintf("%x", sumMD5.Sum(nil))

	if err := extractMeta(&d); err != nil {
		return d, err
	}

	return d, nil
}

func main() {
	flag.Parse()

	c := struct {
		prospect.Archive
		prospect.Context
		Exif []string
		Data []Data `toml:"file"`
	}{}

	if err := toml.DecodeFile(flag.Arg(0), &c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	run(c.Data, func(d Data) {
		log.Printf("looking for files in %s", d.File)
		filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
			now := time.Now()
			if err != nil || i.IsDir() || !d.Accept(file) {
				return err
			}
			log.Printf("start processing %s", d.File)
			d, err := d.Process(file)
			if err != nil {
				log.Printf("fail to process %s: %s", d.File, err)
				return nil
			}
			k, err := c.CreateFromCommand(d.Data, d.Archive, c.Exif)
			if err != nil {
				log.Printf("fail to execute command %s %s: %s", strings.Join(c.Exif, " "), d.File, err)
				return err
			}
			d.Links = append(d.Links, k)
			if err := c.Store(d.Data, d.Archive); err != nil {
				log.Printf("fail to store %s: %s", d.File, err)
				return nil
			}
			log.Printf("done processing %s (%d - %s - %s)", d.File, d.Size, time.Since(now), d.MD5)
			return nil
		})
	})
}

func extractMeta(d *Data) error {
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

	a := prospect.MakeParameter("video.duration", p.Length())
	d.Parameters = append(d.Parameters, a)

	return nil
}

func run(data []Data, fn func(Data)) {
	for _, d := range data {
		if d.Mime == Mime {
			fn(d)
		}
	}
}
