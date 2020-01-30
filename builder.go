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

	"github.com/midbel/toml"
)

type Builder struct {
	meta     Meta
	data     Data
	modules  []Config
	schedule Schedule

	dryrun bool

	marshaler
}

func NewBuilder(file, schedule string) (*Builder, error) {
	c := struct {
		Archive string
		Dry     bool `toml:"no-data"`
		Path    string

		Meta
		Data    `toml:"dataset"`
		Plugins []Config `toml:"module"`
	}{}
	if err := toml.DecodeFile(file, &c); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(c.Archive), 0755); err != nil {
		return nil, err
	}
	m, err := newMarshaler(c.Archive, c.Path)
	if err != nil {
		return nil, err
	}

	b := Builder{
		dryrun:    c.Dry,
		meta:      c.Meta,
		data:      c.Data,
		modules:   c.Plugins,
		marshaler: m,
	}
	if b.data.Model == "" {
		b.data.Model = DefaultModel
	}
	b.meta.Starts = b.meta.Starts.UTC()
	b.meta.Ends = b.meta.Ends.UTC()

	b.schedule, err = loadSchedule(schedule, b.meta.Starts, b.meta.Ends)
	return &b, err
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
	resolve, err := Parse(cfg.Path)
	if err != nil && cfg.Path != "" {
		return err
	}
	for {
		switch i, err := mod.Process(); err {
		case nil:
			src, ps := b.schedule.Keep(i)
			if src == "" {
				continue
			}
			i.Parameters = append(i.Parameters, ps...)

			x := b.data
			x.Experiment = b.meta.Name
			x.Source = src
			x.Info = i

			if len(i.Related) > 0 {
				// chicken/egg problem
			}

			if err := b.marshalData(x, resolve); err != nil {
				return err
			}
			if !b.dryrun {
				if err := b.copyFile(x, resolve); err != nil {
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

type marshaler interface {
	copyFile(Data, resolver) error

	marshalData(Data, resolver) error
	marshalMeta(Meta) error
}

func newMarshaler(file, path string) (marshaler, error) {
	r, err := Parse(path)
	if err != nil {
		return nil, err
	}

	ext := filepath.Ext(file)
	if i, _ := os.Stat(file); ext == "" || i.IsDir() {
		f := filebuilder{
			rootdir:  file,
			resolver: r,
		}
		return &f, nil
	}

	w, err := os.Create(file)
	if err != nil {
		return nil, err
	}

	z := zipbuilder{
		Closer:   w,
		writer:   zip.NewWriter(w),
		resolver: r,
	}
	z.writer.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(w, flate.BestCompression)
	})
	return &z, nil
}

type filebuilder struct {
	rootdir string
	resolver
}

func (b *filebuilder) copyFile(d Data, rs resolver) error {
	r, err := os.Open(d.Info.File)
	if err != nil {
		return err
	}
	defer r.Close()

	file := b.prepareFile(d, rs)
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

func (b *filebuilder) marshalData(d Data, rs resolver) error {
	file := b.prepareFile(d, rs)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	w, err := os.Create(file + ".xml")
	if err != nil {
		return err
	}
	defer w.Close()

	if d.Info.File != file {
		dir := b.rootdir
		if !strings.HasSuffix(dir, "/") {
			dir += "/"
		}
		d.Info.File = strings.TrimPrefix(file, dir)
	}

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

func (b *filebuilder) prepareFile(d Data, r resolver) string {
	var file string
	if r == nil {
		r = b.resolver
	}
	if dir := r.Resolve(d); dir != "" {
		file = filepath.Join(b.rootdir, dir, filepath.Base(d.Info.File))
	} else {
		file = filepath.Join(b.rootdir, d.Rootdir, d.Info.File)
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

func (b *zipbuilder) copyFile(d Data, rs resolver) error {
	r, err := os.Open(d.Info.File)
	if err != nil {
		return err
	}
	defer r.Close()

	file := b.prepareFile(d, rs)
	fh := zip.FileHeader{
		Name:     file,
		Modified: d.Info.AcqTime,
	}
	w, err := b.writer.CreateHeader(&fh)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	return err
}

func (b *zipbuilder) marshalData(d Data, rs resolver) error {
	file := b.prepareFile(d, rs)
	fh := zip.FileHeader{
		Name:     file + ".xml",
		Modified: d.Info.AcqTime,
	}
	w, err := b.writer.CreateHeader(&fh)
	if err != nil {
		return err
	}

	if d.Info.File != file {
		d.Info.File = file
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

func (b *zipbuilder) prepareFile(d Data, r resolver) string {
	var file string
	if r == nil {
		r = b.resolver
	}
	if dir := r.Resolve(d); dir != "" {
		file = filepath.Join(d.Rootdir, dir, filepath.Base(d.Info.File))
	} else {
		file = filepath.Join(d.Rootdir, d.Info.File)
	}
	return file
}
