package main

import (
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/busoc/prospect"
	"github.com/busoc/rt"
	"github.com/midbel/glob"
)

const (
	fileDuration = "file.duration"
	fileRecord   = "file.numrec"
	fileSize     = "file.size"
	pktCorrupted = "pkt.corrupted"
)

type module struct {
	cfg prospect.Config

	buf    []byte
	digest hash.Hash
	source *glob.Glob
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
	return m, err
}

func (m module) String() string {
	return "rt"
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
			i.Mime = prospect.MimeOctetUnformat
		}
		if i.Type == "" {
			i.Mime = prospect.TypeRawTelemetry
		}
		i.Integrity = m.cfg.Integrity
	} else {
		err = fmt.Errorf("%s: %s", file, err)
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

	ps, err := m.readFile(io.TeeReader(rt.NewReader(r), m.digest))
	if err == nil {
		i.File = file
		i.Sum = fmt.Sprintf("%x", m.digest.Sum(nil))
		i.AcqTime = timeFromFile(file)
		i.Parameters = ps

		s, err := r.Stat()
		if err == nil {
			i.ModTime = s.ModTime().UTC()
		}
	}
	return i, err
}

func (m module) readFile(rs io.Reader) ([]prospect.Parameter, error) {
	var size int64
	for i := 0; ; i++ {
		if n, err := rs.Read(m.buf); err != nil {
			ps := []prospect.Parameter{
				prospect.MakeParameter(fileDuration, "300s"),
				prospect.MakeParameter(fileRecord, fmt.Sprintf("%d", i)),
				prospect.MakeParameter(fileSize, fmt.Sprintf("%d", size)),
			}
			if err != io.EOF {
				ps = append(ps, prospect.MakeParameter(pktCorrupted, fmt.Sprintf("%t", err != io.EOF)))
			}
			return ps, nil
		} else {
			size += int64(n)
		}
	}
}

func timeFromFile(file string) time.Time {
	return prospect.TimeRT(file)
}
