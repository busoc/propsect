package prospect

import (
	"archive/zip"
	"compress/flate"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/midbel/toml"
)

const (
	FileSize     = "file.size"
	FileRecords  = "file.numrec"
	FileMD5      = "file.md5"
	FileDuration = "file.duration"

	PtrRef  = "ptr.%d.href"
	PtrRole = "ptr.%d.role"
)

type Builder struct {
	meta     Meta
	data     Data
	modules  []Config
	schedule Schedule

	path   string
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
	m, err := newMarshaler(c.Archive)
	if err != nil {
		return nil, err
	}

	b := Builder{
		dryrun:    c.Dry,
		path:      c.Path,
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
		if m.Path == "" {
			m.Path = b.path
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

var (
	setInfos = []byte("infos")
	setFiles = []byte("files")
)

func (b *Builder) executeModule(mod Module, cfg Config) error {
	file := filepath.Join(os.TempDir(), mod.String()+"-cache.db")
	db, err := bolt.Open(file, 0644, nil)
	if err != nil {
		return err
	}
	defer func() {
		db.Close()
		os.Remove(file)
	}()
	db.Update(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists(setInfos)
		tx.CreateBucketIfNotExists(setFiles)
		return nil
	})

	if err := b.gatherInfo(db, mod, cfg); err != nil {
		return err
	}
	return b.buildArchive(db)
}

func (b *Builder) gatherInfo(db *bolt.DB, mod Module, cfg Config) error {
	resolve, err := cfg.resolver()
	if err != nil {
		return err
	}
	for seed := 0; ; seed++ {
		switch i, err := mod.Process(); err {
		case nil:
			src, ps := b.schedule.Keep(i)
			if src == "" {
				break
			}
			i.Parameters = append(i.Parameters, ps...)

			x := b.data
			x.Experiment = b.meta.Name
			x.Source = src
			x.Info = i

			err := db.Update(func(tx *bolt.Tx) error {
				var (
					key  = keyForFile(x.Info.File)
					bis  = tx.Bucket(setInfos)
					bfs  = tx.Bucket(setFiles)
					file = filepath.Join(resolve.Resolve(x), filepath.Base(x.Info.File))
				)
				xs, err := json.Marshal(x)
				if err != nil {
					return err
				}

				if err := bis.Put(key, xs); err != nil {
					return err
				}
				return bfs.Put(key, []byte(file))
			})
			if err != nil {
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

func (b *Builder) buildArchive(db *bolt.DB) error {
	return db.View(func(tx *bolt.Tx) error {
		var (
			bis = tx.Bucket(setInfos)
			bfs = tx.Bucket(setFiles)
		)
		return bis.ForEach(func(key, value []byte) error {
			var (
				d Data
				j int
			)
			if err := json.Unmarshal(value, &d); err != nil {
				return err
			}
			for _, k := range d.Info.Links {
				file := bfs.Get(keyForFile(k.File))
				if file == nil {
					k.Role = TypeUnavailable
				} else {
					k.File = string(file)
				}
				j++
				ps := []Parameter{
					MakeParameter(fmt.Sprintf(PtrRef, j), k.File),
					MakeParameter(fmt.Sprintf(PtrRole, j), k.Role),
				}
				d.Info.Parameters = append(d.Info.Parameters, ps...)
			}
			original := d.Info.File
			if file := bfs.Get(key); file != nil {
				d.Info.File = string(file)
			}
			if err := b.marshalData(d); err != nil {
				return err
			}
			if b.dryrun {
				return nil
			}
			return b.copyFile(original, d)
		})
	})
}

func keyForFile(file string) []byte {
	file = filepath.Base(file)
	for {
		e := filepath.Ext(file)
		if e == "" {
			break
		}
		file = strings.TrimSuffix(file, e)
	}
	return []byte(file)
}

type marshaler interface {
	copyFile(string, Data) error

	marshalData(Data) error
	marshalMeta(Meta) error
}

func newMarshaler(file string) (marshaler, error) {
	ext := filepath.Ext(file)
	if i, _ := os.Stat(file); ext == "" || i.IsDir() {
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
	fmt.Println("copying", r.Name(), w.Name(), err)
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

	// if d.Info.File != file {
	// 	d.Info.File = file
	// }

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
