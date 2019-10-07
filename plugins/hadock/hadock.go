package main

import (
	"github.com/busoc/prospect"
)

type module struct {
	dir     string
	pattern string
}

func New() prospect.Module {
	return nil
}

func (m module) Process(file string) (prospect.FileInfo, error) {
	var i prospect.FileInfo
	return i, nil
}
