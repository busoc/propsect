package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/busoc/prospect"
	"github.com/midbel/glob"
)

const (
	fileChannel  = "hpkt.vmu2.hci"
	fileOid      = "hpkt.vmu2.origin"
	fileSource   = "hpkt.vmu2.source"
	fileUPI      = "hpkt.vmu2.upi"
	fileInstance = "hpkt.vmu2.instance"
	fileMode     = "hpkt.vmu2.mode"
	fileFCC      = "hpkt.vmu2.fmt"
	fileWidth    = "hpkt.vmu2.pixels.x"
	fileHeight   = "hpkt.vmu2.pixels.y"
	fileBad      = "hpkt.vmu2.invalid"
	fileRoiOffX  = "hpkt.vmu2.roi.xof"
	fileRoiSizX  = "hpkt.vmu2.roi.xsz"
	fileRoiOffY  = "hpkt.vmu2.roi.yof"
	fileRoiSizY  = "hpkt.vmu2.roi.ysz"
	fileDrop     = "hpkt.vmu2.fdrp"
	fileScalSizX = "hpkt.vmu2.scale.xsz"
	fileScalSizY = "hpkt.vmu2.scale.ysz"
	fileScalFar  = "hpkt.vmu2.scale.far"

	scienceRun = "scienceRun"
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

type metadata struct {
	PktTime string `xml:"vmu,attr"`

	AcqTime string `xml:"timestamp"`
	SizeX   int    `xml:"pixels>x"`
	SizeY   int    `xml:"pixels>y"`

	Region struct {
		OffsetX int `xml:"offset-x"`
		SizeX   int `xml:"size-x"`
		OffsetY int `xml:"offset-y"`
		SizeY   int `xml:"size-y"`
	} `xml:"region"`

	Dropping int

	Scale struct {
		SizeX int `xml:"size-x"`
		SizeY int `xml:"size-y"`
	} `xml:"scaling"`
	Ratio int `xml:"force-aspect-ratio"`
}

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
	return "hadock"
}

func (m module) Indexable() bool {
	return false
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
		i.Level = m.cfg.Level
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
	if s, err := r.Stat(); err == nil && s.IsDir() {
		return i, prospect.ErrSkip
	}

	var rs io.Reader = r
	if filepath.Ext(file) == prospect.ExtGZ {
		r, err := gzip.NewReader(rs)
		if err != nil {
			return i, err
		}
		defer r.Close()
		rs = r
	}

	c := prospect.NewCounter()
	rs = io.TeeReader(rs, c)

	if ps, err := readFile(io.TeeReader(rs, m.digest)); err == nil {
		i.File = file
		i.Sum = fmt.Sprintf("%x", m.digest.Sum(nil))
		i.AcqTime = timeFromFile(file)

		if upi, xs, err := parseFilename(file); err == nil {
			ps = append(ps, xs...)
			i.Run = upi
		}
		if s, err := r.Stat(); err == nil {
			i.ModTime = s.ModTime().UTC()
		}
		if r, err := os.Open(file + ".xml"); err == nil {
			defer r.Close()
			var m metadata
			if err := xml.NewDecoder(r).Decode(&m); err != nil {
				return prospect.FileInfo{}, err
			}
			ps = append(ps, prospect.MakeParameter(fileRoiOffX, strconv.Itoa(m.Region.OffsetX)))
			ps = append(ps, prospect.MakeParameter(fileRoiSizX, strconv.Itoa(m.Region.SizeX)))
			ps = append(ps, prospect.MakeParameter(fileRoiOffY, strconv.Itoa(m.Region.OffsetY)))
			ps = append(ps, prospect.MakeParameter(fileRoiSizY, strconv.Itoa(m.Region.SizeY)))
			ps = append(ps, prospect.MakeParameter(fileDrop, strconv.Itoa(m.Dropping)))
			ps = append(ps, prospect.MakeParameter(fileScalSizX, strconv.Itoa(m.Scale.SizeX)))
			ps = append(ps, prospect.MakeParameter(fileScalSizY, strconv.Itoa(m.Scale.SizeY)))
			ps = append(ps, prospect.MakeParameter(fileScalFar, strconv.Itoa(m.Ratio)))
		}
		i.Parameters = append(i.Parameters, ps...)
		i.Parameters = append(i.Parameters, c.AsParameter())
		if filepath.Ext(file) == prospect.ExtGZ {
			i.Parameters = append(i.Parameters, prospect.MakeParameter(prospect.FileEncoding, prospect.MimeGZ))
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

	ps = append(ps, prospect.MakeParameter(fileFCC, strings.TrimSpace(string(buf[:4]))))
	if isImage(buf[:4]) {
		var (
			x = binary.BigEndian.Uint16(buf[16:])
			y = binary.BigEndian.Uint16(buf[18:])
		)
		ps = append(ps, prospect.MakeParameter(fileWidth, fmt.Sprintf("%d", x)))
		ps = append(ps, prospect.MakeParameter(fileHeight, fmt.Sprintf("%d", y)))
	}

	io.Copy(ioutil.Discard, rs)
	return ps, nil
}

func parseFilename(file string) (string, []prospect.Parameter, error) {
	dir, file := filepath.Split(file)
	parts := strings.Split(file, "_")

	ps := make([]prospect.Parameter, 0, 10)
	src, ok := sources[parts[0]]
	if ok {
		ps = append(ps, prospect.MakeParameter(fileSource, src))
	}
	ps = append(ps, prospect.MakeParameter(fileOid, strings.TrimLeft(parts[0], "0")))

	var upi []string
	for i := 1; i < len(parts)-5; i++ {
		upi = append(upi, parts[i])
	}
	ps = append(ps, prospect.MakeParameter(fileUPI, strings.Join(upi, "_")))
	ps = append(ps, prospect.MakeParameter(scienceRun, strings.Join(upi, "_")))

	switch parts[2] {
	case "1", "2":
		ps = append(ps, prospect.MakeParameter(fileChannel, chanVic+parts[2]))
	case "3":
		ps = append(ps, prospect.MakeParameter(fileChannel, chanLrsd))
	default:
	}

	switch {
	case strings.Index(dir, "playback") >= 0:
		ps = append(ps, prospect.MakeParameter(fileMode, "playback"))
	case strings.Index(dir, "realtime") >= 0:
		ps = append(ps, prospect.MakeParameter(fileMode, "realtime"))
	}

	switch {
	case strings.Index(dir, "OPS") >= 0:
		ps = append(ps, prospect.MakeParameter(fileInstance, "OPS"))
	case strings.Index(dir, "SIM1") >= 0:
		ps = append(ps, prospect.MakeParameter(fileInstance, "SIM1"))
	case strings.Index(dir, "SIM2") >= 0:
		ps = append(ps, prospect.MakeParameter(fileInstance, "SIM2"))
	case strings.Index(dir, "TEST") >= 0:
		ps = append(ps, prospect.MakeParameter(fileInstance, "TEST"))
	}

	if bad := filepath.Ext(file) == ".bad"; bad {
		ps = append(ps, prospect.MakeParameter(fileBad, fmt.Sprintf("%t", bad)))
	}

	return strings.Join(upi, "_"), ps, nil
}

func timeFromFile(file string) time.Time {
	return prospect.TimeHadock(file)
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
