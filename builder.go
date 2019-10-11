package prospect

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/midbel/toml"
)

type Builder struct {
	meta    Meta
	data    Data
	mimes   []Mime
	modules []Config
	sources []Activity

	io.Closer
	writer *zip.Writer
}

func NewBuilder(file string) (*Builder, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	c := struct {
		Archive string

		Meta
		Data    `toml:"dataset"`
		Mimes   []Mime     `toml:"mimetype"`
		Plugins []Config   `toml:"module"`
		Periods []Activity `toml:"period"`
	}{}
	if err := toml.NewDecoder(r).Decode(&c); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(c.Archive), 0755); err != nil {
		return nil, err
	}
	writer, err := os.Create(c.Archive)
	if err != nil {
		return nil, err
	}
	b := Builder{
		meta:    c.Meta,
		data:    c.Data,
		mimes:   c.Mimes,
		modules: c.Plugins,
		sources: c.Periods,
		Closer:  writer,
		writer:  zip.NewWriter(writer),
	}
	return &b, nil
}

func (b *Builder) Close() error {
	defer b.Closer.Close()
	return b.writer.Close()
}

func (b *Builder) Build() error {
	for _, m := range b.modules {
		if m.Integrity == "" {
			m.Integrity = b.data.Integrity
		}
		if m.Type == "" {
			m.Type = b.data.Type
		}
		mod, err := m.Open()
		if err != nil {
			return err
		}
		for {
			switch i, err := mod.Process(); err {
			case nil:
				src, keep := b.keepFile(i)
				if !keep {
					continue
				}
				i.Mime = b.guessType(filepath.Ext(i.File))

				x := b.data
				x.Experiment = b.meta.Name
				x.Source = src
				x.Info = i

				if err := b.marshalData(x); err != nil {
					return err
				}
				if err := b.copyFile(x); err != nil {
					return err
				}
			case ErrSkip:
			case ErrDone:
				return nil
			default:
				return fmt.Errorf("%s: %s", mod, err)
			}
		}
	}
	return b.marshalMeta()
}

func (b *Builder) marshalMeta() error {
	fh := zip.FileHeader{
		Name:     fmt.Sprintf("MD_EXP_%s.xml", b.meta.Accr),
		Modified: time.Now().UTC(),
	}
	w, err := b.writer.CreateHeader(&fh)
	if err != nil {
		return err
	}
	return encodeMeta(w, b.meta)
}

func (b *Builder) marshalData(d Data) error {
	fh := zip.FileHeader{
		Name:     filepath.Join(d.Rootdir, d.Info.File) + ".xml",
		Modified: d.Info.AcqTime,
	}
	w, err := b.writer.CreateHeader(&fh)
	if err != nil {
		return err
	}
	return encodeData(w, d)
}

func (b *Builder) copyFile(d Data) error {
	r, err := os.Open(d.Info.File)
	if err != nil {
		return err
	}
	defer r.Close()

	fh := zip.FileHeader{
		Name:     filepath.Join(d.Rootdir, d.Info.File),
		Modified: d.Info.AcqTime,
	}
	w, err := b.writer.CreateHeader(&fh)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	return err
}

func (b *Builder) keepFile(i FileInfo) (string, bool) {
	if len(b.sources) == 0 {
		return "", true
	}
	if i.File == "" {
		return "", false
	}
	var src string
	for _, s := range b.sources {
		if i.AcqTime.After(s.Starts) && i.AcqTime.Before(s.Ends) {
			if src = s.Type; src == "" {
				src = b.data.Source
			}
			return src, true
		}
	}
	return "", false
}

func (b *Builder) guessType(ext string) string {
	mime := DefaultMime
	for _, m := range b.mimes {
		t, ok := m.Has(ext)
		if ok {
			mime = t
			break
		}
	}
	return mime
}
