package main

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/busoc/prospect"
	"github.com/busoc/prospect/cmd/internal/trace"
	"github.com/midbel/mime"
)

const (
	MainType = "application"
	SubType  = "octet-stream"
	typType  = "hpkt-vmu2"
	imgType  = "image"
	scType   = "science"
)

var epoch = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

func main() {
	skipbad := flag.Bool("skip-bad", false, "don't process files with bad extension")
	flag.Parse()

	accept := func(d prospect.Data) bool {
		if d.Level == 1 {
			return d.Mime == prospect.MimePng || d.Mime == prospect.MimeJpeg
		}
		mt, err := mime.Parse(d.Mime)
		if err != nil {
			return false
		}
		if mt.MainType != MainType && mt.SubType != SubType {
			return false
		}
		var (
			typ = strings.ToLower(mt.Params["type"])
			sub = strings.ToLower(mt.Params["subtype"])
		)
		return typ == typType && (sub == imgType || sub == scType)
	}
	err := prospect.Build(flag.Arg(0), collectData(*skipbad), accept)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func collectData(skipbad bool) prospect.RunFunc {
	return func (b prospect.Builder, d prospect.Data) {
		tracer := trace.New("mkhdk")
		defer tracer.Summarize()
		filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
			if err != nil || i.IsDir() {
				return err
			}
			if filepath.Ext(file) == ".xml" {
				return nil
			}
			dat := d.Clone()

			tracer.Start(file)
			dat, err = processData(dat, file)
			if err != nil {
				tracer.Error(file, err)
				return nil
			}
			if err := b.Store(dat); err != nil {
				tracer.Error(file, err)
				return nil
			}
			tracer.Done(file, dat)
			return nil
		})
	}
}


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
	modeRT   = "realtime"
	modePB   = "playback"
	modeOPS  = "OPS"
	modeSIM1 = "SIM1"
	modeSIM2 = "SIM2"
	modeTEST = "TEST"
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

func processData(d prospect.Data, file string) (prospect.Data, error) {
	d.File = file
	if err := prospect.ReadFile(&d, file); err != nil {
		return d, err
	}
	if d.Level == 0 {
		if err := updateDataRaw(&d); err != nil {
			return d, err
		}
	}
	if err := updateMetadataFromXML(&d); err != nil {
		return d, err
	}
	if err := updateMetadataFromName(&d); err != nil {
		return d, err
	}
	return d, nil
}

func updateDataRaw(d *prospect.Data) error {
	r, err := prospect.OpenFile(d.File)
	if err != nil {
		return err
	}
	defer r.Close()

	c := struct {
		FCC    [4]byte
		Seq    uint32
		Unix   uint64
		Width  uint16
		Height uint16
	}{}
	if err := binary.Read(r, binary.BigEndian, &c); err != nil {
		return err
	}
	d.AcqTime = epoch.Add(time.Duration(c.Unix))
	d.ModTime = epoch.Add(time.Duration(c.Unix))

	fcc := prospect.MakeParameter(fileFCC, strings.TrimSpace(string(c.FCC[:])))
	d.Parameters = append(d.Parameters, fcc)
	if isImage(c.FCC[:]) {
		ps := []prospect.Parameter{
			prospect.MakeParameter(prospect.ImageWidth, c.Width),
			prospect.MakeParameter(prospect.ImageHeight, c.Height),
		}
		d.Parameters = append(d.Parameters, ps...)
	}
	return nil
}

func updateMetadataFromName(d *prospect.Data) error {
	parseFilename(d)
	parseDirname(d)
	return nil
}

func parseFilename(d *prospect.Data) {
	parts := strings.Split(filepath.Base(d.File), "_")

	if src, ok := sources[parts[0]]; ok {
		d.Register(fileSource, src)
	}
	d.Register(fileOid, strings.TrimLeft(parts[0], "0"))

	var upi []string
	for i := 1; i < len(parts)-5; i++ {
		upi = append(upi, parts[i])
	}
	d.Register(fileUPI, strings.Join(upi, "_"))
	d.Register(scienceRun, strings.Join(upi, "_"))

	switch parts[2] {
	case "1", "2":
		d.Register(fileChannel, chanVic+parts[2])
	case "3":
		d.Register(fileChannel, chanLrsd)
	default:
	}

	if bad := filepath.Ext(d.File) == ".bad"; bad {
		d.Register(prospect.FileInvalid, bad)
	}

	if d.AcqTime.IsZero() {
		when, _ := time.Parse("20060102150405", parts[len(parts)-3]+parts[len(parts)-2])
		d.AcqTime = when
		d.ModTime = when
	}
}

func parseDirname(d *prospect.Data) {
	dir := filepath.Dir(d.File)
	switch {
	case strings.Index(dir, modePB) >= 0:
		d.Register(fileMode, modePB)
	case strings.Index(dir, modeRT) >= 0:
		d.Register(fileMode, modeRT)
	}

	switch {
	case strings.Index(dir, modeOPS) >= 0:
		d.Register(fileInstance, modeOPS)
	case strings.Index(dir, modeSIM1) >= 0:
		d.Register(fileInstance, modeSIM1)
	case strings.Index(dir, modeSIM2) >= 0:
		d.Register(fileInstance, modeSIM2)
	case strings.Index(dir, modeTEST) >= 0:
		d.Register(fileInstance, modeTEST)
	}
}

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

func updateMetadataFromXML(d *prospect.Data) error {
	r, err := os.Open(d.File + ".xml")
	if err != nil {
		return nil
	}
	defer r.Close()

	var m metadata
	if err := xml.NewDecoder(r).Decode(&m); err != nil {
		return err
	}
	d.Register(fileRoiOffX, m.Region.OffsetX)
	d.Register(fileRoiSizX, m.Region.SizeX)
	d.Register(fileRoiOffY, m.Region.OffsetY)
	d.Register(fileRoiSizY, m.Region.SizeY)
	d.Register(fileDrop, m.Dropping)
	d.Register(fileScalSizX, m.Scale.SizeX)
	d.Register(fileScalSizY, m.Scale.SizeY)
	d.Register(fileScalFar, m.Ratio)

	return nil
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
