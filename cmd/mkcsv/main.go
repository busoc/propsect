package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/busoc/prospect"
	"github.com/busoc/prospect/cmd/internal/trace"
	"github.com/midbel/mime"
)

const (
	MainType   = "text"
	SubType    = "csv"
	fileHeader = "file.%d.header"
)

const TimePattern = "2006-01-02T15:04:05.000"

func main() {
	flag.Parse()

	accept := func(d prospect.Data) bool {
		m, err := mime.Parse(d.Mime)
		if err != nil {
			return false
		}
		return m.MainType == MainType && m.SubType == SubType
	}
	err := prospect.Build(flag.Arg(0), collectData, accept)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func collectData(b prospect.Builder, d prospect.Data) {
	tracer := trace.New("mkcsv")
	defer tracer.Summarize()
	filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
		if err != nil || i.IsDir() || !d.Accept(file) {
			return err
		}
		dat := d.Clone()
		tracer.Start(file)
		if dat, err = processData(dat, file); err != nil {
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

func processData(d prospect.Data, file string) (prospect.Data, error) {
	if err := prospect.ReadFile(&d, file); err != nil {
		return d, err
	}
	return readFile(d)
}

func readFile(d prospect.Data) (prospect.Data, error) {
	r, err := prospect.OpenFile(d.File)
	if err != nil {
		return d, err
	}

	rs := csv.NewReader(r)
	rs.Comma = getDelimiter(d.Mime)
	rs.ReuseRecord = true

	row, err := rs.Read()
	fmt.Println(row, err)
	if err != nil {
		return d, err
	}
	for i := range row {
		p := prospect.MakeParameter(fmt.Sprintf(fileHeader, i+1), row[i])
		d.Parameters = append(d.Parameters, p)
	}

	var count int
	for {
		row, err := rs.Read()
		if len(row) == 0 {
			break
		}
		if err != nil {
			return d, err
		}
		if count == 0 {
			d.AcqTime, err = time.Parse(TimePattern, row[0])
		}
		d.ModTime, err = time.Parse(TimePattern, row[0])
		count++
	}
	delta := d.ModTime.Sub(d.AcqTime)
	d.Parameters = append(d.Parameters, prospect.MakeParameter(prospect.FileDuration, delta))
	d.Parameters = append(d.Parameters, prospect.MakeParameter(prospect.FileRecord, count))
	return d, nil
}

func getDelimiter(str string) rune {
	mt, err := mime.Parse(str)
	if err != nil {
		return ','
	}

	switch mt.Params["delimiter"] {
	default:
		return ','
	case "tab", "\t":
		return '\t'
	case "space", " ":
		return ' '
	case "pipe", "|":
		return '|'
	case "colon", ":":
		return ':'
	case "semicolon", ";":
		return ';'
	}
}
