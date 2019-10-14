package main

import (
	"fmt"
	"hash"
	"io"
	"os"

	"github.com/busoc/prospect"
	"github.com/midbel/glob"
)

const (
	fileSize = "file.size"
)

type module struct {
	cfg prospect.Config

	digest hash.Hash
	source *glob.Globber
}

func New(cfg prospect.Config) prospect.Module {
	return module{
		cfg:    cfg,
		digest: cfg.Hash(),
		source: glob.New("", cfg.Location),
	}
}

func (m module) String() string {
	return "basic"
}

func (m module) Process() (prospect.FileInfo, error) {
	file := m.source.Glob()
	fmt.Println(file)
	if file == "" {
		return prospect.FileInfo{}, prospect.ErrDone
	}
	i, err := m.process(file)
	if err == nil {
		i.Type = m.cfg.Type
		i.Integrity = m.cfg.Integrity
	}
	return i, err
}

func (m module) process(file string) (prospect.FileInfo, error) {
	var i prospect.FileInfo

	r, err := os.Open(file)
	if err != nil {
		return i, err
	}
	defer func() {
		r.Close()
		m.digest.Reset()
	}()

	if _, err := io.Copy(m.digest, r); err != nil {
		return i, err
	}
	i.File = file
	i.Sum = fmt.Sprintf("%x", m.digest.Sum(nil))

	s, err := r.Stat()
	if err == nil {
		i.AcqTime = s.ModTime().UTC()
		i.ModTime = s.ModTime().UTC()
		p := prospect.Parameter{
			Name:  fileSize,
			Value: fmt.Sprintf("%d", s.Size()),
		}
		i.Parameters = append(i.Parameters, p)
	}
	return i, err
}
