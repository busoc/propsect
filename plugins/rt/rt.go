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
	fileDuration  = "file.duration"
	fileRecord    = "file.numrec"
	fileSize      = "file.size"
	fileCorrupted = "file.corrupted"
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
	file := m.source.Glob()
	if file == "" {
		return prospect.FileInfo{}, prospect.ErrDone
	}
	return m.process(file)
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
		i.Integrity = m.cfg.Integrity
		i.Sum = fmt.Sprintf("%x", m.digest.Sum(nil))
		i.AcqTime = timeFromFile(file)
		i.Parameters = ps

		s, err := r.Stat()
		if err == nil {
			i.ModTime = s.ModTime()
		}
	}
	return i, err
}

func (m module) readFile(rs io.Reader) ([]prospect.Parameter, error) {
	var size int64
	for i := 0; ; i++ {
		switch n, err := rs.Read(m.buf); err {
		case nil:
			size += int64(n)
		case io.EOF, rt.ErrInvalid:
			ps := []prospect.Parameter{
				{Name: fileDuration, Value: "300s"},
				{Name: fileRecord, Value: fmt.Sprintf("%d", i)},
				{Name: fileSize, Value: fmt.Sprintf("%d", size)},
				{Name: fileCorrupted, Value: fmt.Sprintf("%t", err == rt.ErrInvalid)},
			}
			return ps, nil
		default:
			return nil, err
		}
	}
}

func timeFromFile(file string) time.Time {
	var (
		parts = make([]int, 3)
		dir   = filepath.Dir(file)
		base  = filepath.Base(file)
		when  time.Time
	)
	for i := len(parts) - 1; i >= 0; i-- {
		d, f := filepath.Split(dir)
		x, err := strconv.Atoi(f)
		if err != nil {
			return when
		}
		parts[i] = x
		dir = filepath.Dir(d)
	}
	when = when.AddDate(parts[0]-1, 0, parts[1]).Add(time.Duration(parts[2]) * time.Hour)
	if x := strings.Index(base, "_"); x >= 0 {
		base = base[x+1:]
		if x = strings.Index(base, "_"); x >= 0 {
			x, err := strconv.Atoi(base[:x])
			if err == nil {
				when = when.Add(time.Duration(x) * time.Minute)
			}
		}
	}
	return when.UTC()
}
