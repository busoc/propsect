package main

import (
	"hash"
	"time"

	"github.com/busoc/prospect"
	"github.com/midbel/glob"
)

const (
	fileChannel  = "file.channel"
	fileSource   = "file.source"
	fileUPI      = "file.upi"
	fileInstance = "file.instance"
	fileMode     = "file.mode"
	fileFCC      = "file.fcc"
	fileWidth    = "file.pixels.x"
	fileHeight   = "file.pixels.y"
)

type module struct {
	cfg prospect.Config

	buf    []byte
	digest hash.Hash
	source *glob.Globber
}

func New(cfg prospect.Config) prospect.Module {
	m := module{
		cfg:    cfg,
		buf:    make([]byte, 8<<20),
		digest: cfg.Hash(),
		source: glob.New("", cfg.Location),
	}
	return m
}

func (m module) Process() (prospect.FileInfo, error) {
	var file string
	for {
		file = m.source.Glob()
		if file == "" {
			return prospect.FileInfo{}, prospect.ErrDone
		}
		if filepath.Ext(file) == ".xml" {
			continue
		}
	}
	i, err := m.process(file)
	if err == nil {

	}
	return i, err
}

func parseFilename(file string) ([]prospect.Parameter, error) {
	return nil, nil
}

func timeFromFile(file string) time.Time {
	return time.Now()
}
