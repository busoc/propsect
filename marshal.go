package prospect

import (
	"archive/zip"
	"compress/flate"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type marshaler interface {
	copyFile(string, Data) error

	marshalData(Data) error
	marshalMeta(Meta) error
}

func newMarshaler(file string) (marshaler, error) {
	ext := filepath.Ext(file)
	if i, err := os.Stat(file); ext == "" || (err == nil && i.IsDir()) {
		f := filebuilder{
			rootdir: file,
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
}

func (b *filebuilder) copyFile(file string, d Data) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	file = filepath.Join(b.rootdir, d.Info.File)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}

	w, err := os.Create(file)
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = io.Copy(w, r)
	return err
}

func (b *filebuilder) marshalData(d Data) error {
	file := filepath.Join(b.rootdir, d.Info.File)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
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

func (b *zipbuilder) marshalData(d Data) error {
	fh := zip.FileHeader{
		Name:     d.Info.File + ".xml",
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
