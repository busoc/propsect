package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/busoc/prospect"
	"github.com/midbel/glob"
)

const (
	fileSize     = "file.size"
	fileChannel  = "hrd.channel"
	fileSource   = "hrd.source"
	fileUPI      = "hrd.upi"
	fileInstance = "hrd.instance"
	fileMode     = "hrd.mode"
	fileFCC      = "hrd.fcc"
	fileWidth    = "hrd.pixels.x"
	fileHeight   = "hrd.pixels.y"
	fileBad      = "hrd.invalid"
)

const (
	chanVic  = "vic"
	chanLrsd = "lrsd"
)

var sources = map[string]string{
	"0033": "Thermoteknix 640FF",
	"0034": "mikrotron EoSens Cub6",
	"0035": "RUBI experiment science parameters",
	"0036": "VMU/RUBI synchronization unit",
	"0037": "Point Grey FL3-GE-50S5M-C",
	"0038": "Basier Racer raL2048-48gm",
	"0039": "EC/ALV CorrTector #1",
	"0040": "EC/ALV CorrTector #2",
	"0041": "VMU/SMD synchronization unit",
	"0042": "virtual camera #1",
	"0043": "virtual camera #2",
	"0044": "virtual camera #3",
	"0045": "virtual camera #4",
	"0046": "virtual camera #5",
	"0047": "virtual camera #6",
	"0051": "Low Rate Science Data",
	"0090": "svs",
}

var (
	MMA  = []byte("MMA ")
	CORR = []byte("CORR")
	SYNC = []byte("SYNC")
	RAW  = []byte("RAW ")
	Y800 = []byte("Y800")
	Y16B = []byte("Y16 ")
	Y16L = []byte("Y16L")
	I420 = []byte("I420")
	YUY2 = []byte("YUY2")
	RGB  = []byte("RGB ")
	JPEG = []byte("JPEG")
	PNG  = []byte("PNG ")
	H264 = []byte("H264")
	SVS  = []byte("SVS ")
	TIFF = []byte("TIFF")
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

func (m module) String() string {
	return "hadock"
}

func (m module) Process() (prospect.FileInfo, error) {
	var file string
	for {
		file = m.source.Glob()
		if file == "" {
			return prospect.FileInfo{}, prospect.ErrDone
		}
		if filepath.Ext(file) == ".xml" {
			continue
		}
		break
	}
	i, err := m.process(file)
	switch err {
	case nil:
		i.Mime, i.Type = m.cfg.GuessType(filepath.Ext(file))
		if i.Mime == "" {
			i.Mime = prospect.MimeOctetDefault
		}
		if i.Type == "" {
			i.Type = prospect.TypeHighRateData
		}
		i.Integrity = m.cfg.Integrity
	case prospect.ErrSkip:
	default:
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
	if s, _ := r.Stat(); err == nil && s.IsDir() {
		return i, prospect.ErrSkip
	}
	if ps, err := readFile(io.TeeReader(r, m.digest)); err == nil {
		i.File = file
		i.Sum = fmt.Sprintf("%x", m.digest.Sum(nil))
		i.AcqTime = timeFromFile(file)

		i.Parameters = ps
		if ps, err = parseFilename(file); err == nil {
			i.Parameters = append(i.Parameters, ps...)
		}
		if s, err := r.Stat(); err == nil {
			i.ModTime = s.ModTime().UTC()
		}
	}
	return i, nil
}

func readFile(rs io.Reader) ([]prospect.Parameter, error) {
	var (
		buf = make([]byte, 20)
		ps  = make([]prospect.Parameter, 0, 10)
	)

	if _, err := rs.Read(buf); err != nil {
		return nil, err
	}

	ps = append(ps, newParameter(fileFCC, strings.TrimSpace(string(buf[:4]))))
	if isImage(buf[:4]) {
		var (
			x = binary.BigEndian.Uint16(buf[16:])
			y = binary.BigEndian.Uint16(buf[18:])
		)
		ps = append(ps, newParameter(fileWidth, fmt.Sprintf("%d", x)))
		ps = append(ps, newParameter(fileHeight, fmt.Sprintf("%d", y)))
	}

	n, err := io.Copy(ioutil.Discard, rs)
	if err == nil {
		size := int(n) + len(buf)
		ps = append(ps, newParameter(fileSize, fmt.Sprintf("%d", size)))
	}
	return ps, nil
}

func parseFilename(file string) ([]prospect.Parameter, error) {
	dir, file := filepath.Split(file)
	parts := strings.Split(file, "_")

	ps := make([]prospect.Parameter, 0, 10)
	if src, ok := sources[parts[0]]; ok {
		ps = append(ps, newParameter(fileSource, src))
	}
	ps = append(ps, newParameter(fileUPI, parts[1]))

	switch parts[2] {
	case "1", "2":
		ps = append(ps, newParameter(fileChannel, chanVic+parts[2]))
	case "3":
		ps = append(ps, newParameter(fileChannel, chanLrsd))
	default:
	}

	switch {
	case strings.Index(dir, "playback") >= 0:
		ps = append(ps, newParameter(fileMode, "playback"))
	case strings.Index(dir, "realtime") >= 0:
		ps = append(ps, newParameter(fileMode, "realtime"))
	}

	switch {
	case strings.Index(dir, "OPS") >= 0:
		ps = append(ps, newParameter(fileInstance, "OPS"))
	case strings.Index(dir, "SIM1") >= 0:
		ps = append(ps, newParameter(fileInstance, "SIM1"))
	case strings.Index(dir, "SIM2") >= 0:
		ps = append(ps, newParameter(fileInstance, "SIM2"))
	case strings.Index(dir, "TEST") >= 0:
		ps = append(ps, newParameter(fileInstance, "TEST"))
	}

	bad := filepath.Ext(file) == ".bad"
	ps = append(ps, newParameter(fileBad, fmt.Sprintf("%t", bad)))

	return ps, nil
}

func timeFromFile(file string) time.Time {
	ps := strings.Split(filepath.Base(file), "_")

	when, _ := time.Parse("20060102150405", ps[len(ps)-3]+ps[len(ps)-2])
	return when
}

func isImage(fcc []byte) bool {
	switch {
	default:
		return true
	case bytes.Equal(fcc, MMA):
	case bytes.Equal(fcc, CORR):
	case bytes.Equal(fcc, SYNC):
	case bytes.Equal(fcc, RAW):
	case bytes.Equal(fcc, SVS):
	}
	return false
}

func newParameter(k, v string) prospect.Parameter {
	return prospect.Parameter{Name: k, Value: v}
}
