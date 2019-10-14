package main

import (
	"encoding/csv"
	"hash"
	"io"
	"os"

	"github.com/busoc/prospect"
)

type module struct {
	cfg prospect.Config

	io.Closer
	reader *csv.Reader
	digest hash.Hash
}

func New(cfg prospect.Config) (prospect.Module, error) {
	r, err := os.Open(cfg.Location)
	if err != nil {
		return nil, err
	}
	m := &module{
		cfg:    cfg,
		digest: cfg.Hash(),
		Closer: r,
		reader: csv.NewReader(r),
	}
	m.reader.ReuseRecord = true
	m.reader.FieldsPerRecord = 9

	return m, nil
}

func (m *module) String() string {
	return "filup"
}

func (m *module) Process() (prospect.FileInfo, error) {
	row, err := m.reader.Read()
	switch err {
	case nil:
	case io.EOF:
		m.Closer.Close()
		return prospect.FileInfo{}, prospect.ErrDone
	default:
		return prospect.FileInfo{}, err
	}
	i, err := m.process(row)
	if err == nil {

	}
	return i, err
}

func (m *module) process(row []string) (prospect.FileInfo, error) {
	return prospect.FileInfo{}, nil
}
