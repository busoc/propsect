package prospect

import (
	"archive/zip"
	"compress/flate"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
		Dry     bool     `toml:"no-data"`
		Levels  []string `toml:"directories"`

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
	m, err := newMarshaler(c.Archive, c.Levels)
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
			if m, t := cfg.guessType(ext); m == "" {
				i.Mime, i.Type = b.guessType(ext)
			} else {
				i.Mime, i.Type = m, t
			}
			if i.Type == "" {
				i.Type = b.data.Type
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

func (b *Builder) guessType(ext string) (string, string) {
	var (
		mime = DefaultMime
		data string
	)
	for _, m := range b.mimes {
		mi, ty, ok := m.Has(ext)
		if ok {
			mime, data = mi, ty
			break
		}
	}
	return mime, data
}

type marshaler interface {
	copyFile(Data) error

	marshalData(Data) error
	marshalMeta(Meta) error
}

func newMarshaler(file string, levels []string) (marshaler, error) {
	ext := filepath.Ext(file)
	if i, _ := os.Stat(file); ext == "" || i.IsDir() {
		return &filebuilder{
			rootdir: file,
			dirtree: dirtree(levels),
		}, nil
	}

	w, err := os.Create(file)
	if err != nil {
		return nil, err
	}
	z := zipbuilder{
		Closer:  w,
		writer:  zip.NewWriter(w),
		dirtree: dirtree(levels),
	}
	z.writer.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(w, flate.BestCompression)
	})
	return &z, nil
}

type filebuilder struct {
	rootdir string
	dirtree
}

func (b *filebuilder) copyFile(d Data) error {
	r, err := os.Open(d.Info.File)
	if err != nil {
		return err
	}
	defer r.Close()

	file := b.prepareFile(d)
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
	file := b.prepareFile(d)
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

func (b *filebuilder) prepareFile(d Data) string {
	var file string
	if dir := b.Prepare(d); dir != "" {
		file = filepath.Join(b.rootdir, dir, filepath.Base(d.Info.File))
	} else {
		file = filepath.Join(b.rootdir, d.Rootdir, d.Info.File)
	}
	return file
}

type zipbuilder struct {
	io.Closer
	writer *zip.Writer

	dirtree
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

	file := b.prepareFile(d)
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

func (b *zipbuilder) marshalData(d Data) error {
	file := b.prepareFile(d)
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

func (b *zipbuilder) prepareFile(d Data) string {
	var file string
	if dir := b.Prepare(d); dir != "" {
		file = filepath.Join(d.Rootdir, dir, filepath.Base(d.Info.File))
	} else {
		file = filepath.Join(d.Rootdir, d.Info.File)
	}
	return file
}

const (
	levelSource = "source"
	levelModel  = "model"
	levelMime   = "mime"
	levelFormat = "format"
	levelType   = "type"
	levelYear   = "year"
	levelDoy    = "doy"
	levelMonth  = "month"
	levelDay    = "day"
	levelHour   = "hour"
	levelMin    = "minute"
	levelSec    = "second"
	levelStamp  = "timestamp"
)

type dirtree []string

func (d dirtree) Prepare(dat Data) string {
	if len(d) == 0 {
		return ""
	}
	return d.prepare(dat)
}

func (d dirtree) prepare(dat Data) string {
	replace := func(str string) string {
		return strings.ReplaceAll(strings.Title(str), " ", "")
	}
	parts := make([]string, len(d))
	for i, p := range d {
		switch p {
		case levelSource:
			parts[i] = replace(dat.Source)
		case levelModel:
			parts[i] = replace(dat.Model)
		case levelMime, levelFormat:
			parts[i] = replace(parseMime(dat.Info.Mime))
		case levelType:
			parts[i] = replace(dat.Info.Type)
		case levelYear:
			parts[i] = strconv.Itoa(dat.Info.AcqTime.Year())
		case levelDoy:
			parts[i] = fmt.Sprintf("%03d", dat.Info.AcqTime.YearDay())
		case levelMonth:
			parts[i] = fmt.Sprintf("%02d", dat.Info.AcqTime.Month())
		case levelDay:
			parts[i] = fmt.Sprintf("%02d", dat.Info.AcqTime.Day())
		case levelHour:
			parts[i] = fmt.Sprintf("%02d", dat.Info.AcqTime.Hour())
		case levelMin:
			parts[i] = fmt.Sprintf("%02d", dat.Info.AcqTime.Minute())
		case levelSec:
			parts[i] = fmt.Sprintf("%02d", dat.Info.AcqTime.Second())
		case levelStamp:
			u := dat.Info.AcqTime.Unix()
			parts[i] = strconv.Itoa(int(u))
		default:
			parts[i] = p
		}
	}
	return filepath.Join(parts...)
}

func parseMime(mime string) string {
	if ix := strings.Index(mime, "/"); ix >= 0 && ix+1 < len(mime) {
		mime = mime[ix+1:]
	}
	if ix := strings.Index(mime, ";"); ix >= 0 {
		mime = mime[:ix]
	}
	return mime
}
