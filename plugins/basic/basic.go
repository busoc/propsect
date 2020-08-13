package main

import (
	"compress/gzip"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"

	"github.com/busoc/prospect"
	"github.com/midbel/glob"
)

type module struct {
	cfg prospect.Config

	timefn prospect.TimeFunc
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

	m.timefn, err = cfg.GetTimeFunc()
	return m, err
}

func (m module) String() string {
	return "basic"
}

func (m module) Indexable() bool {
	return false
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
		i.Level = m.cfg.Level
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

	var rs io.Reader = r
	if filepath.Ext(file) == ".gz" {
		r, err := gzip.NewReader(rs)
		if err != nil {
			return i, err
		}
		defer r.Close()
		rs = r
	}

	if _, err := io.Copy(m.digest, rs); err != nil {
		return i, err
	}
	i.File = file
	i.Sum = fmt.Sprintf("%x", m.digest.Sum(nil))

	s, err := r.Stat()
	if err == nil {
		if m.timefn != nil {
			i.AcqTime = m.timefn(file)
		} else {
			i.AcqTime = s.ModTime().UTC()
		}
		i.ModTime = s.ModTime().UTC()

		p := prospect.MakeParameter(prospect.FileSize, fmt.Sprintf("%d", s.Size()))
		i.Parameters = append(i.Parameters, p)
	}
	return i, err
}
