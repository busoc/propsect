package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/busoc/prospect"
	"github.com/busoc/rt"
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

	r, err := os.Open(file)
	if err != nil {
		return i, err
	}
	defer r.Close()
	digest := sha256.New()

	ps, err := readFile(io.TeeReader(rt.NewReader(r), digest))
	if err == nil {
		i.Sum = fmt.Sprintf("%x", digest.Sum(nil))
		i.AcqTime = timeFromFile(file)
		i.Parameters = ps

		s, err := r.Stat()
		if err == nil {
			i.ModTime = s.ModTime()
		}
	}
	return i, err
}

func readFile(rs io.Reader) ([]prospect.Parameter, error) {
	var (
		size int64
		buf  = make([]byte, 8<<20)
	)
	for i := 0; ; i++ {
		switch n, err := rs.Read(buf); err {
		case nil:
			size += int64(n)
		case io.EOF, rt.ErrInvalid:
			ps := []prospect.Parameter{
				{Name: "file.numrec", Value: fmt.Sprintf("%d", i)},
				{Name: "file.size", Value: fmt.Sprintf("%d", size)},
				{Name: "file.corrupted", Value: fmt.Sprintf("%d", err == rt.ErrInvalid)},
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
