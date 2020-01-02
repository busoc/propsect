package main

import (
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"

	"github.com/busoc/prospect"
	"github.com/midbel/glob"
)

const (
	fileSize = "file.size"
)

type module struct {
	cfg prospect.Config

	digest hash.Hash
	source *glob.Glob
}

func New(cfg prospect.Config) (prospect.Module, error) {
	m := module{
		cfg:    cfg,
		digest: cfg.Hash(),
	}

	g, err := glob.New(cfg.Location)
	if err == nil {
		m.source = g
	}
	return m, err
}

func (m module) String() string {
	return "basic"
}

func (m module) Process() (prospect.FileInfo, error) {
	file := m.source.Glob()
	if file == "" {
		return prospect.FileInfo{}, prospect.ErrDone
	}
	i, err := m.process(file)
	if err == nil {
		i.Mime, i.Type = m.cfg.GuessType(filepath.Ext(file))
		if i.Mime == "" {
			i.Mime = prospect.MimeOctetDefault
		}
		if i.Type == "" {
			i.Type = prospect.TypeData
		}
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
