package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/busoc/prospect"
)

const (
	fileSize      = "file.size"
	fileMD5       = "file.md5"
	fileOriginal  = "file.filename.original"
	fileUplink    = "file.filename.uplink"
	fileMMU       = "file.filename.mmu"
	fileUpTime    = "file.time.uplink"
	fileFerTime   = "file.time.transfer"
	fileSource    = "file.source"
	fileRecords   = "file.numrec"
	fileReference = "file.reference"
)

type module struct {
	cfg prospect.Config

	io.Closer
	reader *csv.Reader
	digest hash.Hash

	next    []string
	sources map[string]struct{}
}

func New(cfg prospect.Config) (prospect.Module, error) {
	r, err := os.Open(cfg.Location)
	if err != nil {
		return nil, err
	}
	m := &module{
		cfg:     cfg,
		digest:  cfg.Hash(),
		Closer:  r,
		reader:  csv.NewReader(r),
		sources: make(map[string]struct{}),
	}
	m.reader.ReuseRecord = true
	m.reader.FieldsPerRecord = 9

	return m, nil
}

func (m *module) String() string {
	return "filup"
}

func (m *module) Process() (prospect.FileInfo, error) {
	var (
		row []string
		err error
	)

	if m.next == nil {
		row, err = m.reader.Read()
		switch err {
		case nil:
		case io.EOF:
			m.Closer.Close()
			return prospect.FileInfo{}, prospect.ErrDone
		default:
			return prospect.FileInfo{}, err
		}
		if _, ok := m.sources[row[0]]; !ok {
			m.sources[row[0]] = struct{}{}
			m.next = row
		}
		return m.processListing(row[0])
	} else {
		row, m.next = m.next, nil
	}

	i, err := m.process(row)
	switch err {
	case nil:
		i.Type = m.cfg.Type
		i.Integrity = m.cfg.Integrity
	case prospect.ErrSkip:
	default:
		err = fmt.Errorf("%s: %s", row[1], err)
	}
	return i, err
}

func (m *module) processListing(file string) (prospect.FileInfo, error) {
	var i prospect.FileInfo

	r, err := os.Open(file)
	if err != nil {
		return i, err
	}
	defer func() {
		r.Close()
		m.digest.Reset()
	}()

	scan := bufio.NewScanner(io.TeeReader(r, m.digest))

	var (
		refs []string
		size int
	)
	for scan.Scan() {
		row := scan.Text()
		if strings.HasPrefix(row, "Filename:") {
			refs = append(refs, strings.TrimSpace(strings.TrimPrefix(row, "Filename:")))
		}
		size += len(row)
	}
	if err := scan.Err(); err != nil {
		return i, err
	}

	s, err := r.Stat()
	if err != nil {
		return i, err
	}
	i.Parameters = []prospect.Parameter{
		newParameter(fileSize, fmt.Sprintf("%d", size)),
		newParameter(fileRecords, fmt.Sprintf("%d", len(refs))),
	}
	dir := filepath.Dir(file)
	for _, r := range refs {
		i.Parameters = append(i.Parameters, newParameter(fileReference, filepath.Join(dir, r)))
	}

	i.AcqTime = s.ModTime().UTC()
	i.ModTime = s.ModTime().UTC()
	i.Sum = fmt.Sprintf("%x", m.digest.Sum(nil))
	i.File = r.Name()

	return i, err
}

func (m *module) process(row []string) (prospect.FileInfo, error) {
	var i prospect.FileInfo

	dir := filepath.Dir(row[0])
	r, err := os.Open(filepath.Join(dir, row[1]))
	if err != nil {
		return i, prospect.ErrSkip
	}
	defer func() {
		r.Close()
		m.digest.Reset()
	}()

	if _, err = io.Copy(m.digest, r); err != nil {
		return i, err
	}
	s, err := r.Stat()
	if err != nil {
		return i, err
	}
	i.Parameters = []prospect.Parameter{
		newParameter(fileSource, row[0]),
		newParameter(fileOriginal, row[2]),
		newParameter(fileUplink, row[1]),
		newParameter(fileMMU, row[3]),
		newParameter(fileMD5, row[8]),
		newParameter(fileSize, fmt.Sprintf("%d", s.Size())),
	}
	if row[5] != "" || row[5] != "-" {
		i.Parameters = append(i.Parameters, newParameter(fileUpTime, row[5]))
	}
	if row[6] != "" || row[6] != "-" {
		i.Parameters = append(i.Parameters, newParameter(fileFerTime, row[6]))
	}

	i.AcqTime = s.ModTime().UTC()
	i.ModTime = s.ModTime().UTC()
	i.Sum = fmt.Sprintf("%x", m.digest.Sum(nil))
	i.File = r.Name()

	return i, nil
}

func newParameter(n, v string) prospect.Parameter {
	return prospect.Parameter{
		Name:  n,
		Value: v,
	}
}
