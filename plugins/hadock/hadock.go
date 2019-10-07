package main

import (
	"github.com/busoc/prospect"
)

const (
	fileChannel  = "file.channel"
	fileSource   = "file.source"
	fileUPI      = "file.upi"
	fileInstance = "file.instance"
	fileMode     = "file.mode"
)

type module struct {
	dir     string
	pattern string
}

func New() prospect.Module {
	return nil
}

func (m module) Process() (prospect.FileInfo, error) {
	var i prospect.FileInfo
	return i, nil
}
