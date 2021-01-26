package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/busoc/prospect"
	"github.com/busoc/rt"
	"github.com/busoc/timutil"
	"github.com/midbel/mime"
)

func main() {
	flag.Parse()

	accept := func(d prospect.Data) bool {
		mt, err := mime.Parse(d.Mime)
		if err != nil {
			fmt.Println(mt.Params, mt.MainType, mt.SubType, err)
			return false
		}
		switch strings.ToLower(mt.Params["type"]) {
		case "pth", "pdh", "hrd":
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
	buffer := make([]byte, 8<<20)
	filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
		if err != nil || i.IsDir() || !d.Accept(file) {
			return err
		}
		now := time.Now()

		log.Printf("start processing %s", d.File)
		d, err := processData(d, file, buffer)
		if err != nil {
			log.Printf("fail to update %s: %s", d.File, err)
			return nil
		}
		if err := b.Store(d); err != nil {
			log.Printf("fail to store %s: %s", d.File, err)
			return nil
		}
		log.Printf("done %s (%d - %s - %s)", d.File, d.Size, time.Since(now), d.MD5)
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
				d.Parameters = append(d.Parameters, prospect.MakeParameter(prospect.FileInvalid, true))
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
	d.Parameters = append(d.Parameters, prospect.MakeParameter(prospect.FileDuration, delta))
	d.Parameters = append(d.Parameters, prospect.MakeParameter(prospect.FileRecord, count))
	return d, nil
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
		AcqCoarse uint32
		AcqFine   uint8
		RecCoarse uint32
		RecFine   uint8
	}{}
	if err := binary.Read(bytes.NewReader(buf), binary.BigEndian, &c); err != nil {
		return time.Time{}
	}
	return timutil.Join5(c.AcqCoarse, c.AcqFine)
}
