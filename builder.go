package prospect

import (
	"archive/zip"
	"compress/flate"
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

	dryrun bool

	marshaler
}

func NewBuilder(file string) (*Builder, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	c := struct {
		Archive string
		Dry     bool `toml:"no-data"`

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
	m, err := newMarshaler(c.Archive)
	if err != nil {
		return nil, err
	}

	b := Builder{
		dryrun:    c.Dry,
		meta:      c.Meta,
		data:      c.Data,
		mimes:     c.Mimes,
		modules:   c.Plugins,
		sources:   c.Periods,
		marshaler: m,
	}
	return &b, nil
}

func (b *Builder) Close() error {
	if c, ok := b.marshaler.(io.Closer); ok {
		return c.Close()
	}
	return nil
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
		if err := b.executeModule(mod, m); err != nil {
			return err
		}
	}
	return b.marshalMeta(b.meta)
}

func (b *Builder) executeModule(mod Module, cfg Config) error {
	for {
		switch i, err := mod.Process(); err {
		case nil:
			src, keep := b.keepFile(i)
			if !keep {
				continue
			}
			ext := filepath.Ext(i.File)
			if m := cfg.guessType(ext); m == "" {
				i.Mime = b.guessType(ext)
			} else {
				i.Mime = m
			}

			x := b.data
			x.Experiment = b.meta.Name
			x.Source = src
			x.Info = i

			if err := b.marshalData(x); err != nil {
				return err
			}
			if !b.dryrun {
				if err := b.copyFile(x); err != nil {
					return err
				}
			}
		case ErrSkip:
		case ErrDone:
			return nil
		default:
			return fmt.Errorf("%s: %s", mod, err)
		}
	}
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

type marshaler interface {
	copyFile(Data) error

	marshalData(Data) error
	marshalMeta(Meta) error
}

func newMarshaler(file string) (marshaler, error) {
	ext := filepath.Ext(file)
	if i, _ := os.Stat(file); ext == "" || i.IsDir() {
		return &filebuilder{rootdir: file}, nil
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

func (b *filebuilder) copyFile(d Data) error {
	r, err := os.Open(d.Info.File)
	if err != nil {
		return err
	}
	defer r.Close()

	file := filepath.Join(b.rootdir, d.Rootdir, d.Info.File)
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
	file := filepath.Join(b.rootdir, d.Rootdir, d.Info.File) + ".xml"
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	w, err := os.Create(file)
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
}

func (b *zipbuilder) Close() error {
	err := b.writer.Close()
	if e := b.Closer.Close(); e != nil && err == nil {
		err = e
	}
	return err
}

func (b *zipbuilder) copyFile(d Data) error {
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

func (b *zipbuilder) marshalData(d Data) error {
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
