package main

import (
	"github.com/busoc/prospect"
)

const (
	fileDuration = "file.duration"
	fileRecord   = "file.numrec"
	fileSize     = "file.size"
	fileHeaders  = "file.headers"
)

type module struct {
}

func New(cfg prospect.Config) (prospect.Module, error) {
	return nil, nil
}

func (m *module) String() string {
	return "csv"
}

func (m *module) Process() (prospect.FileInfo, error) {
	return prospect.FileInfo{}, nil
}
