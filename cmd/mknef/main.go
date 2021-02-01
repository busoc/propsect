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
	"github.com/busoc/prospect/cmd/internal/trace"
	"github.com/midbel/exif/nef"
)

const (
	Mime = "image/x-nikon-nef"

	ExtDAT = ".dat"
	ExtJPG = ".jpg"
)

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
	tracer := trace.New("mknef")
	defer tracer.Summarize()
	filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
		if err != nil || i.IsDir() || !d.Accept(file) {
			return err
		}
		dat := d.Clone()

		tracer.Start(file)
		defer tracer.Done(file, dat)

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

		d.AcqTime = dat.AcqTime
		d.ModTime = dat.ModTime
		d.Links = append(d.Links, prospect.CreateLinkFrom(dat))
		extractImages(file, func(base string, f *nef.File) error {
			d := dat.Clone()
			buf, err := updateDataFromImage(f, &d)
			if err != nil {
				tracer.Error(file, err)
				return err
			}
			k, err := b.CreateFile(d, buf)
			if err == nil {
				k.Role = d.Type
				dat.Links = append(dat.Links, k)
			}
			return err
		})
		if err := b.Store(dat); err != nil {
			tracer.Error(file, err)
		}
		return nil
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
		d.Register(prospect.ImageWidth, cfg.Width)
		d.Register(prospect.ImageHeight, cfg.Height)
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

func extractImages(file string, fn func(string, *nef.File) error) error {
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
