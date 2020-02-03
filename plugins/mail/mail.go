package main

import (
	"github.com/busoc/prospect"
)

type module struct{}

func New(cfg prospect.Config) (prospect.Module, error) {
	return module{}, nil
}

func (m module) Process() (prospect.FileInfo, error) {
	return prospect.FileInfo{}, nil
}
