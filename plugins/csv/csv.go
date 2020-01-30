package main

import (
	"github.com/busoc/prospect"
)

const (
	fileDuration = prospect.FileDuration
	fileRecord   = prospect.FileRecords
	fileSize     = prospect.FileSize
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
