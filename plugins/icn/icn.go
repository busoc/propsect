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
	"time"

	"github.com/busoc/prospect"
)

const (
	fileMMU     = "uplink.target.path"
	fileUpTime  = "uplink.time.uplink"
	fileFerTime = "uplink.time.transfer"
	fileRecords = "file.numrec"

	uplinkRole = "uplinked file"
	icnRole    = prospect.TypeUplinkNote
)

const timePattern = "2006/002 15:04"

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
	m.reader.FieldsPerRecord = 11

	return m, nil
}

func (m *module) String() string {
	return "icn"
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

			return m.startList(row)
		}
	} else {
		row = m.next
	}
	m.next = nil
	return m.startRecord(row)
}

func (m *module) startList(row []string) (prospect.FileInfo, error) {
	i, err := m.processListing(row[0], row[6])
	if err == nil {
		i.Mime, i.Type = m.cfg.GuessType(filepath.Ext(row[0]))
		if i.Mime == "" {
			i.Mime = prospect.MimeICN
		}
		if i.Type == "" {
			i.Type = prospect.TypeUplinkNote
		}
		i.Integrity = m.cfg.Integrity
	}
	return i, err
}

func (m *module) startRecord(row []string) (prospect.FileInfo, error) {
	i, err := m.processRecord(row)
	switch err {
	case nil:
		i.Mime, i.Type = m.cfg.GuessType(filepath.Ext(row[1]))
		if i.Mime == "" {
			i.Mime = prospect.MimePlainDefault
		}
		if i.Type == "" {
			i.Type = prospect.TypeUplinkFile
		}
		i.Integrity = m.cfg.Integrity
	case prospect.ErrSkip:
	default:
		err = fmt.Errorf("%s: %s", row[1], err)
	}
	return i, err
}

func (m *module) processRecord(row []string) (prospect.FileInfo, error) {
	var i prospect.FileInfo

	dir := filepath.Dir(row[0])
	r, err := openFile(filepath.Join(dir, row[1]))
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
		prospect.MakeParameter(fileMMU, row[3]),
		prospect.MakeParameter(prospect.FileMD5, row[10]),
		prospect.MakeParameter(prospect.FileSize, fmt.Sprintf("%d", s.Size())),
	}
	i.Links = append(i.Links, prospect.Link{File: filepath.Base(row[0]), Role: icnRole})

	if row[6] != "" || row[6] != "-" {
		i.AcqTime, _ = time.Parse(timePattern, row[6])
		i.AcqTime = i.AcqTime.UTC()
		i.Parameters = append(i.Parameters, prospect.MakeParameter(fileUpTime, row[6]))
	}
	if row[7] != "" || row[7] != "-" {
		i.Parameters = append(i.Parameters, prospect.MakeParameter(fileFerTime, row[7]))
	}

	i.ModTime = s.ModTime().UTC()
	i.Sum = fmt.Sprintf("%x", m.digest.Sum(nil))
	i.File = r.Name()

	return i, nil
}

func (m *module) processListing(file, stamp string) (prospect.FileInfo, error) {
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

	var size int
	for scan.Scan() {
		row := scan.Text()
		size += len(row)
		if !strings.HasPrefix(row, "Filename:") {
			continue
		}
		row = strings.TrimSpace(strings.TrimPrefix(row, "Filename:"))
		i.Links = append(i.Links, prospect.Link{File: row, Role: uplinkRole})
	}
	if err := scan.Err(); err != nil {
		return i, err
	}

	s, err := r.Stat()
	if err != nil {
		return i, err
	}
	i.Parameters = []prospect.Parameter{
		prospect.MakeParameter(prospect.FileSize, fmt.Sprintf("%d", size)),
		prospect.MakeParameter(fileRecords, fmt.Sprintf("%d", len(i.Links))),
	}

	when, err := time.Parse(timePattern, stamp)
	if err != nil {
		when, err = s.ModTime().UTC(), nil
	}
	i.AcqTime = when
	i.ModTime = s.ModTime().UTC()
	i.Sum = fmt.Sprintf("%x", m.digest.Sum(nil))
	i.File = r.Name()

	return i, err
}

func openFile(file string) (*os.File, error) {
	r, err := os.Open(file)
	if err == nil {
		return r, nil
	}
	return os.Open(file + ".DAT")
}
