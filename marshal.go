package prospect

import (
	"archive/zip"
	"compress/flate"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const metadir = "metadata"

type marshaler interface {
	copyFile(string, Data) error

	marshalData(Data, bool) error
	marshalMeta(Meta) error
}

func newMarshaler(file, link string) (marshaler, error) {
	ext := filepath.Ext(file)
	if i, err := os.Stat(file); ext == "" || (err == nil && i.IsDir()) {
		switch strings.ToLower(link) {
		case "":
		case "hard":
		case "soft":
		default:
			return nil, fmt.Errorf("unsupported link type: %s", link)
		}
		f := filebuilder{
			rootdir: file,
			link:    link,
		}
		return &f, nil
	}

	w, err := os.Create(file)
	if err != nil {
		return nil, err
	}

	z := zipbuilder{
		Closer: w,
		writer: zip.NewWriter(w),
	}
	z.writer.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(w, flate.BestCompression)
	})
	return &z, nil
}

type filebuilder struct {
	rootdir string
	link    string
}

func (b *filebuilder) copyFile(file string, d Data) error {
	newfile := filepath.Join(b.rootdir, d.Info.File)
	if err := os.MkdirAll(filepath.Dir(newfile), 0755); err != nil {
		return err
	}
	newfile = updateFile(newfile)
	defer fmt.Fprintln(os.Stderr, file, newfile)
	switch b.link {
	case "soft":
		return os.Symlink(file, newfile)
	case "hard":
		return os.Link(file, newfile)
	default:
		r, err := os.Open(file)
		if err != nil {
			return err
		}
		defer r.Close()

		w, err := os.Create(newfile)
		if err != nil {
			return err
		}
		defer w.Close()

		_, err = io.Copy(w, r)
		return err
	}
}

func (b *filebuilder) marshalData(d Data, samedir bool) error {
	// file := filepath.Join(b.rootdir, d.Info.File)
	file := b.rootdir
	if !samedir {
		file = filepath.Join(file, metadir)
	}
	file = filepath.Join(file, d.Info.File)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	file = updateFile(file)
	w, err := os.Create(file + ".xml")
	if err != nil {
		return err
	}
	defer w.Close()

	return encodeData(w, d)
}

func (b *filebuilder) marshalMeta(m Meta) error {
	file := filepath.Join(b.rootdir, fmt.Sprintf("MD_EXP_%s.xml", m.Accr))
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	w, err := os.Create(file)
	if err != nil {
		return err
	}
	defer w.Close()
	return encodeMeta(w, m)
}

func updateFile(file string) string {
	if ms, _ := filepath.Glob(file + "*"); len(ms) > 0 {
		file = fmt.Sprintf("%s.%d", file, len(ms))
	}
	return file
}

type zipbuilder struct {
	io.Closer
	writer *zip.Writer

	resolver
}

func (b *zipbuilder) Close() error {
	err := b.writer.Close()
	if e := b.Closer.Close(); e != nil && err == nil {
		err = e
	}
	return err
}

func (b *zipbuilder) copyFile(file string, d Data) error {
	defer fmt.Fprintln(os.Stderr, file, d.Info.File)
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	fh := zip.FileHeader{
		Name:     d.Info.File,
		Modified: d.Info.AcqTime,
	}
	w, err := b.writer.CreateHeader(&fh)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	return err
}

func (b *zipbuilder) marshalData(d Data, samedir bool) error {
	file := d.Info.File + ".xml"
	if !samedir {
		file = filepath.Join(metadir, file)
	}
	fh := zip.FileHeader{
		Name:     file,
		Modified: d.Info.AcqTime,
	}
	w, err := b.writer.CreateHeader(&fh)
	if err != nil {
		return err
	}

	return encodeData(w, d)
}

func (b *zipbuilder) marshalMeta(m Meta) error {
	fh := zip.FileHeader{
		Name:     fmt.Sprintf("MD_EXP_%s.xml", m.Accr),
		Modified: time.Now().UTC(),
	}
	w, err := b.writer.CreateHeader(&fh)
	if err != nil {
		return err
	}
	return encodeMeta(w, m)
}
