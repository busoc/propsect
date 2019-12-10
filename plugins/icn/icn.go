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
	fileSize     = "file.size"
	fileMD5      = "file.md5"
	fileOriginal = "uplink.file.local"
	fileUplink   = "uplink.file.uplink"
	fileMMU      = "uplink.file.mmu"
	fileUpTime   = "uplink.time.uplink"
	fileFerTime  = "uplink.time.transfer"
	fileSource   = "uplink.source"
	fileRecords  = "file.numrec"
	ptrRef       = "ptr.%d.href"
	ptrRole      = "ptr.%d.role"

	defaultRole = "uplinked file"
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
		newParameter(fileSource, row[0]),
		newParameter(fileOriginal, row[2]),
		newParameter(fileUplink, row[1]),
		newParameter(fileMMU, row[3]),
		newParameter(fileMD5, row[10]),
		newParameter(fileSize, fmt.Sprintf("%d", s.Size())),
	}
	if row[6] != "" || row[6] != "-" {
		i.AcqTime, _ = time.Parse(timePattern, row[6])
		i.AcqTime = i.AcqTime.UTC()
		i.Parameters = append(i.Parameters, newParameter(fileUpTime, row[6]))
	}
	if row[7] != "" || row[7] != "-" {
		i.Parameters = append(i.Parameters, newParameter(fileFerTime, row[7]))
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

	for j, r := range refs {
		ref := fmt.Sprintf(ptrRef, j+1)
		i.Parameters = append(i.Parameters, newParameter(ref, r))

		rol := fmt.Sprintf(ptrRole, j+1)
		i.Parameters = append(i.Parameters, newParameter(rol, defaultRole))
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

func newParameter(n, v string) prospect.Parameter {
	return prospect.Parameter{
		Name:  n,
		Value: v,
	}
}
