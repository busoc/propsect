package prospect

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	// "path/filepath"
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
	Integrity string
	Module    string
	Location  string
	Type      string
	Mime      string
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

func (c Config) Open() (Module, error) {
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
	fn, ok := sym.(func(Config) Module)
	if !ok {
		return nil, fmt.Errorf("%s: invalid plugin - invalid signture (%T)", c.Module, sym)
	}
	return fn(c), nil
}
