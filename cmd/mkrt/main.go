package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/busoc/prospect"
	"github.com/busoc/prospect/cmd/internal/trace"
	"github.com/busoc/rt"
	"github.com/busoc/timutil"
	"github.com/midbel/mime"
)

const (
	pth = "pth"
	pdh = "pdh"
	hrd = "hrd"
	vmu = "vmu"
)

func main() {
	flag.Parse()

	accept := func(d prospect.Data) bool {
		switch strings.ToLower(d.Type) {
		default:
		case strings.ToLower(prospect.TypePTH), strings.ToLower(prospect.TypePDH), strings.ToLower(prospect.TypeHRD):
			return true
		}
		mt, err := mime.Parse(d.Mime)
		if err != nil {
			return false
		}
		switch strings.ToLower(mt.Params["type"]) {
		case pth, pdh, hrd, vmu:
		default:
			return false
		}
		return true
	}
	err := prospect.Build(flag.Arg(0), collectData, accept)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func collectData(b prospect.Builder, d prospect.Data) {
	var (
		buffer = make([]byte, 8<<20)
		tracer = trace.New("mkrt")
	)
	filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
		if err != nil || i.IsDir() || !d.Accept(file) {
			return err
		}
		dat := d.Clone()

		tracer.Start(file)
		defer tracer.Done(file, dat)
		
		dat, err = processData(dat, file, buffer)
		if err != nil {
			tracer.Error(file, err)
			return nil
		}
		if err := b.Store(dat); err != nil {
			tracer.Error(file, err)
		}
		return nil
	})
}

func processData(d prospect.Data, file string, buffer []byte) (prospect.Data, error) {
	if err := prospect.ReadFile(&d, file); err != nil {
		return d, err
	}
	r, err := prospect.OpenFile(d.File)
	if err != nil {
		return d, err
	}
	defer r.Close()

	var (
		count   int
		rs      = rt.NewReader(r)
		getTime = getTimeFunc(d.Type)
	)

	for i := 0; ; i++ {
		if _, err := rs.Read(buffer); err != nil {
			if !errors.Is(err, io.EOF) {
				d.Register(prospect.FileInvalid, true)
			}
			break
		}
		if i == 0 {
			d.AcqTime = getTime(buffer)
		}
		d.ModTime = getTime(buffer)
		count++
	}
	delta := d.ModTime.Sub(d.AcqTime)
	d.Register(prospect.FileDuration, delta)
	d.Register(prospect.FileRecord, count)
	return d, nil
}

type info struct {
	Id   int
	Seq  int
	When time.Time
}

func getTimeFunc(str string) func([]byte) time.Time {
	switch strings.ToLower(str) {
	default:
		return func(_ []byte) time.Time { return time.Now() }
	case strings.ToLower(prospect.TypePTH):
		return getTimeTM
	case strings.ToLower(prospect.TypePDH):
		return getTimePP
	case strings.ToLower(prospect.TypeHRD):
		return getTimeHRD
	}
}

func getTimePP(buf []byte) time.Time {
	c := struct {
		Size   uint32
		State  uint8
		Orbit  uint32
		Code   [6]byte
		Type   uint8
		Unit   uint16
		Coarse uint32
		Fine   uint8
		Len    uint16
	}{}
	if err := binary.Read(bytes.NewReader(buf), binary.BigEndian, &c); err != nil {
		return time.Time{}
	}
	return timutil.Join5(c.Coarse, c.Fine)
}

func getTimeTM(buf []byte) time.Time {
	c := struct {
		Size      uint32
		RecCoarse uint32
		RecFine   uint8
		Type      uint8
		Pid       uint16
		Seq       uint16
		Len       uint16
		AcqCoarse uint32
		AcqFine   uint8
		Sid       uint32
		Info      uint8
	}{}
	if err := binary.Read(bytes.NewReader(buf), binary.BigEndian, &c); err != nil {
		return time.Time{}
	}
	return timutil.Join5(c.AcqCoarse, c.AcqFine)
}

func getTimeHRD(buf []byte) time.Time {
	c := struct {
		Size      uint32
		Err       uint16
		Payload   uint8
		Channel   uint8
		RecCoarse uint32
		RecFine   uint8
		AcqCoarse uint32
		AcqFine   uint8
	}{}
	if err := binary.Read(bytes.NewReader(buf), binary.BigEndian, &c); err != nil {
		return time.Time{}
	}
	return timutil.Join5(c.AcqCoarse, c.AcqFine)
}
