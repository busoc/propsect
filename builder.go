package prospect

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/midbel/toml"
)

const (
	FileSize     = "file.size"
	FileRecords  = "file.numrec"
	FileMD5      = "file.md5"
	FileDuration = "file.duration"
	FileEncoding = "file.encoding"

	PtrRef  = "ptr.%d.href"
	PtrRole = "ptr.%d.role"
	PtrPath = "ptr.%d.archive"
)

type Builder struct {
	meta     Meta
	data     Data
	modules  []Config
	schedule Schedule

	path    string
	dryrun  bool
	samedir bool

	marshaler
}

func NewBuilder(file, schedule string) (*Builder, error) {
	c := struct {
		Archive string
		Dry     bool `toml:"no-data"`
		Same    bool `toml:"same-dir"`
		Path    string
		Link    string

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
	m, err := newMarshaler(c.Archive, c.Link)
	if err != nil {
		return nil, err
	}

	b := Builder{
		dryrun:    c.Dry,
		samedir:   c.Same,
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

	if mod.Indexable() {
		if err := b.gatherInfo(db, mod, cfg); err != nil {
			return err
		}
		return b.buildArchive(db)
	}
	return b.build(mod, cfg)
}

func (b *Builder) build(mod Module, cfg Config) error {
	resolve, err := cfg.resolver()
	if err != nil {
		return err
	}
	for {
		switch i, err := mod.Process(); err {
		case nil:
			src, ps := b.schedule.Keep(i)
			if src == "" {
				break
			}
			i.Parameters = append(i.Parameters, ps...)

			x := b.data
			x.Experiment = b.meta.Name
			if x.Source == "" {
				x.Source = src
			}
			x.Info = i
			x.Level = i.Level

			original := x.Info.File
			x.Info.File = filepath.Join(resolve.Resolve(x), filepath.Base(x.Info.File))

			if err := b.marshalData(x, b.samedir); err != nil {
				return err
			}
			if b.dryrun {
				return nil
			}
			if err := b.copyFile(original, x); err != nil {
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

func (b *Builder) gatherInfo(db *bolt.DB, mod Module, cfg Config) error {
	resolve, err := cfg.resolver()
	if err != nil {
		return err
	}
	for {
		switch i, err := mod.Process(); err {
		case nil:
			src, ps := b.schedule.Keep(i)
			if src == "" {
				break
			}
			i.Parameters = append(i.Parameters, ps...)
			fmt.Fprintf(os.Stderr, "%s: %s %d %s\n", i.File, i.Sum, i.Size, i.AcqTime.Format("2006-01-02 15:04:05"))

			x := b.data
			x.Experiment = b.meta.Name
			if x.Source == "" {
				x.Source = src
			}
			x.Info = i
			x.Level = i.Level

			err = db.Update(func(tx *bolt.Tx) error {
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
				if file != nil {
					k.File = string(file)
				}
				j++
				ps := []Parameter{
					MakeParameter(fmt.Sprintf(PtrRef, j), k.File),
					MakeParameter(fmt.Sprintf(PtrRole, j), k.Role),
				}
				if file == nil {
					p := MakeParameter(fmt.Sprintf(PtrPath, j), TypeUnavailable)
					ps = append(ps, p)
				}
				d.Info.Parameters = append(d.Info.Parameters, ps...)
			}
			original := d.Info.File
			if file := bfs.Get(key); file != nil {
				d.Info.File = string(file)
			}
			if err := b.marshalData(d, b.samedir); err != nil {
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
