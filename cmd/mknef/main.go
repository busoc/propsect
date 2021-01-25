package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"flag"
	"fmt"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/busoc/prospect"
	"github.com/midbel/exif/nef"
	"github.com/midbel/toml"
)

const (
	SHA  = "SHA256"
	Mime = "image/x-nikon-nef"

	ImgWidth  = "image.width"
	ImgHeight = "image.height"

	ExtDAT = ".dat"
	ExtJPG = ".jpg"
)

type Data struct {
	prospect.Data
}

func (d Data) Process(file string) (Data, error) {
	x, err := d.processNEF(file)
	if err != nil {
		return x, err
	}
	return x, nil
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

	d.Integrity = SHA
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
		d.Integrity = SHA
		filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
			if err != nil || i.IsDir() || !d.Accept(file) {
				return err
			}
			x, err := d.Process(file)
			if err != nil {
				return nil
			}
			k, err := c.CreateFromCommand(x.Data, c.Exif)
			if err != nil {
				return err
			}
			x.Links = append(x.Links, k)
			if err := c.Store(x.Data); err != nil {
				return nil
			}

			d.AcqTime = x.AcqTime
			d.ModTime = x.ModTime
			d.Links = append(d.Links, prospect.CreateLinkFrom(x.Data))
			extractImage(file, func(base string, f *nef.File) error {
				d.Parameters, d.File = d.Parameters[:0], base
				buf, err := updateDataFromImage(f, &d)
				if err != nil {
					return err
				}
				k, err = c.CreateFile(d.Data, buf)
				if err == nil {
					k.Role = d.Type
					x.Links = append(x.Links, k)
				}
				return err
			})
			return c.Store(x.Data)
		})
	})
}

func updateDataFromImage(f *nef.File, d *Data) ([]byte, error) {
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
	d.Size = int64(len(buf))
	d.MD5 = fmt.Sprintf("%x", md5.Sum(buf))
	d.Sum = fmt.Sprintf("%x", sha256.Sum256(buf))
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

func run(data []Data, fn func(Data)) {
	for _, d := range data {
		if d.Mime == Mime {
			fn(d)
		}
	}
}
