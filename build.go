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

type RunFunc func(Builder, Data)

type AcceptFunc func(Data) bool

type Builder struct {
	Include string `toml:"include"`
	Archive
	Context
	Mimes    MimeSet   `toml:"mimetype"`
	Commands []Command `toml:"command"`
	Data     []Data    `toml:"file"`
}

func Build(file string, run RunFunc, accept AcceptFunc) error {
	b, err := Load(file)
	if err != nil {
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
		x.Links = append(x.Links, CreateLinkFrom(d))
		k, err := b.CreateFile(x, buf)
		if err != nil {
			continue
		}
		if k.Role == "" {
			k.Role = TypeCommand
		}
		ks = append(ks, k)
	}
	return ks, nil
}

func Load(file string) (Builder, error) {
	var b Builder
	if err := toml.DecodeFile(file, &b); err != nil {
		return b, err
	}
	if r, err := os.Open(b.Include); err == nil {
		defer r.Close()
		c := struct {
			Archive
			Context
		}{
			Archive: b.Archive,
			Context: b.Context,
		}
		if err := toml.Decode(r, &c); err != nil {
			return b, err
		}
		b.Archive = c.Archive
		b.Context = c.Context
	}
	return b, nil
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
