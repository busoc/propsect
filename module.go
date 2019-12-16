package prospect

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"plugin"
	"strings"
	"time"
)

type FileInfo struct {
	File string
	Type string
	Mime string

	Integrity string
	Sum       string
	Size      int

	ModTime time.Time
	AcqTime time.Time

	Parameters []Parameter
}

type Module interface {
	Process() (FileInfo, error)
	fmt.Stringer
}

type Config struct {
	Integrity   string
	Module      string
	Location    string
	Type        string
	Mime        string
	Mimes       []Mime `toml:"mimetype"`
	Directories dirtree
}

// func (c Config) Prepare(base string) string {
// 	return c.Directories.Prepare(base, nil)
// }

func (c Config) GuessType(ext string) (string, string) {
	for _, m := range c.Mimes {
		if mi, ty, ok := m.Has(ext); ok {
			if ty == "" {
				ty = c.Type
			}
			return mi, ty
		}
	}
	return c.Mime, c.Type
}

func (c Config) Hash() hash.Hash {
	var h hash.Hash
	switch strings.ToLower(c.Integrity) {
	case "sha256", "sha-256":
		h = sha256.New()
	case "sha512", "sha-512":
		h = sha512.New512_256()
	case "md5":
		h = md5.New()
	default:
		h = sha1.New()
	}
	return h
}

func (c Config) Open(levels []string) (Module, error) {
	if c.Module == "" {
		return nil, ErrSkip
	}
	g, err := plugin.Open(c.Module)
	if err != nil {
		return nil, err
	}
	sym, err := g.Lookup("New")
	if err != nil {
		return nil, err
	}
	if len(c.Directories) == 0 {
		c.Directories = append(c.Directories, levels...)
	}
	switch fn := sym.(type) {
	case func(Config) Module:
		return fn(c), nil
	case func(Config) (Module, error):
		return fn(c)
	default:
		return nil, fmt.Errorf("%s: invalid plugin - invalid signture (%T)", c.Module, sym)
	}
}
