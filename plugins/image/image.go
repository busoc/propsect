package main

import (
	"compress/gzip"
	"fmt"
	"hash"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"

	"github.com/busoc/prospect"
	"github.com/midbel/glob"
)

const (
	imgWidth  = "image.width"
	imgHeight = "image.height"
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
	if filepath.Ext(file) == prospect.ExtGZ {
		r, err := gzip.NewReader(rs)
		if err != nil {
			return i, err
		}
		defer r.Close()
		rs = r
	}

	c := prospect.NewCounter()
	rs = io.TeeReader(rs, c)

	if _, err := io.Copy(m.digest, rs); err != nil {
		return i, err
	}
	i.Parameters = append(i.Parameters, c.AsParameter())
	if filepath.Ext(file) == prospect.ExtGZ {
		i.Parameters = append(i.Parameters, prospect.MakeParameter(prospect.FileEncoding, prospect.MimeGZ))
	}

	dims, err := dimension(file)
	if err != nil {
		return i, err
	}
	i.Parameters = append(i.Parameters, dims...)

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
	}
	return i, err
}

func dimensions(file string) ([]prospect.Parameter, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var img io.Reader = r
	if filepath.Ext(file) == prospect.ExtGZ {
		rs, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		img = rs
	}
	cfg, _, err := image.DecodeConfig(img)
	if err != nil {
		return nil, err
	}
	ps := []prospect.Parameter{
		propsect.MakeParameter(imgWidth, fmt.Sprintf("%d", cfg.Width)),
		propsect.MakeParameter(imgHeight, fmt.Sprintf("%d", cfg.Height)),
	}
	return ps, nil
}
