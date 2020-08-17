package main

import (
	"compress/gzip"
	"encoding/csv"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"

	"github.com/busoc/prospect"
	"github.com/midbel/glob"
	"github.com/midbel/mime"
)

const (
	fileHeader = "csv.%d.header"
)

type module struct {
	cfg prospect.Config

	buf    []byte
	digest hash.Hash
	source *glob.Glob

	timefn prospect.TimeFunc
}

func New(cfg prospect.Config) (prospect.Module, error) {
	m := module{
		cfg:    cfg,
		buf:    make([]byte, 8<<20),
		digest: cfg.Hash(),
	}

	g, err := glob.New(cfg.Location)
	if err == nil {
		m.source = g
	}
	m.timefn, err = cfg.GetTimeFunc()
	return &m, err
}

func (m *module) String() string {
	return "csv"
}

func (m *module) Indexable() bool {
	return false
}

func (m *module) Process() (prospect.FileInfo, error) {
	file := m.source.Glob()
	if file == "" {
		return prospect.FileInfo{}, prospect.ErrDone
	}
	i, err := m.process(file)
	if err == nil {
		i.Mime, i.Type = m.cfg.GuessType(filepath.Ext(file))
		if i.Mime == "" {
			i.Mime = prospect.MimeCSV
		}
		if i.Type == "" {
			i.Mime = prospect.TypeData
		}
		i.Integrity = m.cfg.Integrity
		i.Level = m.cfg.Level
	} else {
		if !errors.Is(err, prospect.ErrSkip) {
			err = fmt.Errorf("%s: %s", file, err)
		}
	}
	return i, err
}

func (m *module) process(file string) (prospect.FileInfo, error) {
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

	headers, records, err := m.readFile(rs, m.cfg.Mime)
	if err != nil {
		if err == io.EOF {
			err = prospect.ErrSkip
		}
		return i, err
	}
	//if records == 0 {
	//	return i, prospect.ErrSkip
	//}
	i.File = file
	i.Parameters = []prospect.Parameter{
		prospect.MakeParameter(prospect.FileRecords, fmt.Sprintf("%d", records)),
		c.AsParameter(),
	}
	if filepath.Ext(file) == prospect.ExtGZ {
		i.Parameters = append(i.Parameters, prospect.MakeParameter(prospect.FileEncoding, prospect.MimeGZ))
	}
	for j, h := range headers {
		p := prospect.MakeParameter(fmt.Sprintf(fileHeader, j+1), h)
		i.Parameters = append(i.Parameters, p)
	}

	if s, err := r.Stat(); err == nil {
		if m.timefn != nil {
			i.AcqTime = m.timefn(file)
		} else {
			i.AcqTime = s.ModTime().UTC()
		}
		i.ModTime = s.ModTime()
	}

	return i, nil
}

func (m *module) readFile(r io.Reader, str string) ([]string, int, error) {
	var (
		rs   = csv.NewReader(io.TeeReader(r, m.digest))
		rows int
	)
	mt, err := mime.Parse(str)
	if err != nil {
		return nil, 0, err
	}
	switch mt.Params["delimiter"] {
	case "tab", "\t":
		rs.Comma = '\t'
	case "comma", ",":
		rs.Comma = ','
	case "space", " ":
		rs.Comma = ' '
	case "pipe", "|":
		rs.Comma = '|'
	default:
	}
	hd, err := rs.Read()
	if err != nil {
		return nil, 0, err
	}
	for {
		_, err := rs.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, 0, err
		}
		rows++
	}
	return hd, rows, nil
}
