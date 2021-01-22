package main

import (
	"crypto/md5"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/busoc/prospect"
	"github.com/midbel/exif/nef"
	"github.com/midbel/toml"
)

const Mime = "image/x-nikon-nef"

type Data struct {
	prospect.Data
	Archive string
}

func (d Data) Process(file string) (Data, []Data, error) {
	x, err := d.processNEF(file)
	if err != nil {
		return x, nil, err
	}

	files, err := nef.DecodeFile(file)
	if err != nil {
		return x, nil, err
	}

	var xs []Data
	for _, f := range files {
		ds, err := d.processImage(f)
		if err != nil {
			return x, xs, err
		}
		xs = append(xs, ds...)
	}
	return x, xs, nil
}

func (d Data) processImage(f *nef.File) ([]Data, error) {
	return nil, nil
}

func (d Data) processNEF(file string) (Data, error) {
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

	d.Integrity = "SHA256"
	d.Sum = fmt.Sprintf("%x", sumSHA.Sum(nil))
	d.MD5 = fmt.Sprintf("%x", sumMD5.Sum(nil))

	files, err := nef.DecodeFile(file)
	if err != nil {
		return d, err
	}
	for _, f := range files {
		when, _ := f.GetTag(0x0132, nef.Tiff)
		d.AcqTime = when.Time()
		d.ModTime = when.Time()
		break
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
		d.Data = c.Update(d.Data)
		filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
			if err != nil || i.IsDir() || !d.Accept(file) {
				return err
			}
			d, _, err := d.Process(file)
			if err != nil {
				return nil
			}
			k, err := c.CreateFromCommand(d.Data, d.Archive, c.Exif)
			if err != nil {
				return err
			}
			d.Links = append(d.Links, k)
			if err := c.Store(d.Data, d.Archive); err != nil {
				return nil
			}
			return nil
		})
	})
}

func run(data []Data, fn func(Data)) {
	for _, d := range data {
		if d.Mime == Mime {
			fn(d)
		}
	}
}
