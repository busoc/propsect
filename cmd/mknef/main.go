package main

import (
	"bytes"
	"flag"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"

	"github.com/busoc/prospect"
	"github.com/midbel/exif/nef"
)

const (
	SHA  = "SHA256"
	Mime = "image/x-nikon-nef"

	ImgWidth  = "image.width"
	ImgHeight = "image.height"

	ExtDAT = ".dat"
	ExtJPG = ".jpg"
)

func main() {
	flag.Parse()

	err := prospect.Build(flag.Arg(0), Mime, collectData)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func collectData(b prospect.Builder, d prospect.Data) {
	filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
		if err != nil || i.IsDir() || !d.Accept(file) {
			return err
		}
		x, err := processData(d, file)
		if err != nil {
			return nil
		}
		ks, err := b.ExecuteCommands(x)
		if err != nil {
			return nil
		}
		x.Links = append(x.Links, ks...)
		if err := b.Store(x); err != nil {
			return nil
		}

		d.AcqTime = x.AcqTime
		d.ModTime = x.ModTime
		d.Links = append(d.Links, prospect.CreateLinkFrom(x))
		extractImage(file, func(base string, f *nef.File) error {
			d.Parameters, d.File = d.Parameters[:0], base
			buf, err := updateDataFromImage(f, &d)
			if err != nil {
				return err
			}
			k, err := b.CreateFile(d, buf)
			if err == nil {
				k.Role = d.Type
				x.Links = append(x.Links, k)
			}
			return err
		})
		return b.Store(x)
	})
}

func processData(d prospect.Data, file string) (prospect.Data, error) {
	if err := prospect.ReadFile(&d, file); err != nil {
		return d, err
	}
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

func updateDataFromImage(f *nef.File, d *prospect.Data) ([]byte, error) {
	d.Type = prospect.TypeImage
	d.Mime = prospect.MimeJpeg
	d.Level = 1

	var (
		buf []byte
		err error
		ext string
	)
	if !f.IsSupported() {
		d.Mime = prospect.MimeOctet
		d.Type = prospect.TypeData
		d.Level--

		buf, err = rawBytes(f)
		ext = ExtDAT
	} else {
		buf, err = imageBytes(f)
		ext = ExtJPG

		cfg, _ := jpeg.DecodeConfig(bytes.NewReader(buf))
		d.Parameters = []prospect.Parameter{
			prospect.MakeParameter(ImgWidth, cfg.Width),
			prospect.MakeParameter(ImgHeight, cfg.Height),
		}
	}
	if err != nil {
		return nil, err
	}
	if err := prospect.ReadFrom(d, bytes.NewReader(buf)); err != nil {
		return nil, err
	}
	d.File = d.File + "_" + f.Filename() + ext
	return buf, nil
}

func imageBytes(f *nef.File) ([]byte, error) {
	img, err := f.Image()
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, img, nil)
	return buf.Bytes(), err
}

func rawBytes(f *nef.File) ([]byte, error) {
	return f.Bytes()
}

func extractImage(file string, fn func(string, *nef.File) error) error {
	var (
		base = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		walk func([]*nef.File)
	)
	walk = func(files []*nef.File) {
		for _, f := range files {
			fn(base, f)
			walk(f.Files)
		}
	}
	files, err := nef.DecodeFile(file)
	if err != nil {
		return err
	}
	walk(files)
	return nil
}
