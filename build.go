package prospect

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"

	"github.com/midbel/toml"
)

const ExtGZ = ".gz"

func OpenFile(file string) (io.ReadCloser, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	var r io.Reader = f
	if filepath.Ext(file) == ExtGZ {
		r, err = gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
	}
	rc := readcloser{
		Reader: r,
		closer: f,
	}
	return &rc, nil
}

type Builder struct {
	Archive
	Context
	Mimes    MimeSet   `toml:"mimetype"`
	Commands []Command `toml:"command"`
	Data     []Data    `toml:"file"`
}

type RunFunc func(Builder, Data)

type AcceptFunc func(Data) bool

func Build(file string, run RunFunc, accept AcceptFunc) error {
	var b Builder
	if err := toml.DecodeFile(file, &b); err != nil {
		return err
	}
	if accept == nil {
		accept = func(_ Data) bool { return true }
	}
	for _, d := range b.Data {
		if d.Type == "" && d.Mime == "" && len(b.Mimes) == 0 {
			continue
		}
		if !accept(d) {
			continue
		}
		run(b, b.Update(d))
	}
	return nil
}

func (b Builder) GetMime(d Data) Data {
	m := b.Mimes.Get(filepath.Ext(d.File))
	if m.isZero() {
		return d
	}
	if d.Mime == "" {
		d.Mime = m.Mime
	}
	if d.Type == "" {
		d.Type = m.Type
	}
	return d
}

func (b Builder) ExecuteCommands(d Data) ([]Link, error) {
	var ks []Link
	for _, c := range b.Commands {
		x, buf, err := c.Exec(d)
		if err != nil || len(buf) == 0 {
			continue
		}
		k, err := b.CreateFile(x, buf)
		if err != nil {
			continue
		}
		ks = append(ks, k)
	}
	return ks, nil
}

type readcloser struct {
	io.Reader
	closer io.Closer
}

func (r *readcloser) Close() error {
	if c, ok := r.Reader.(*gzip.Reader); ok {
		c.Close()
	}
	return r.closer.Close()
}
