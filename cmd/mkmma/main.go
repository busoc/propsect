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
	fileHeader = "csv.%d.header"
	scienceRun = "scienceRun.%d"
	scienceRec = "scienceRun.%d.numrec"
	scienceDur = "scienceRun.%d.duration"
)

const DefaultInterval = time.Microsecond * 666

const TimePattern = "2006-01-02T15:04:05.000000"

func main() {
	between := flag.Duration("d", DefaultInterval, "interval of time between two lines")
	flag.Parse()

	accept := func(d prospect.Data) bool {
		m, err := mime.Parse(d.Mime)
		if err != nil {
			return false
		}
		return m.MainType == MainType && m.SubType == SubType
	}
	err := prospect.Build(flag.Arg(0), collectData(*between), accept)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func collectData(between time.Duration) func(prospect.Builder, prospect.Data) {
	return func(b prospect.Builder, d prospect.Data) {
		tracer := trace.New("mkmma")
		defer tracer.Summarize()
		filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
			if err != nil || i.IsDir() || !d.Accept(file) {
				return err
			}
			dat := d.Clone()

			tracer.Start(file)

			if dat, err = processData(dat, file, between); err != nil {
				tracer.Error(file, err)
				return nil
			}
			if err := b.Store(dat); err != nil {
				tracer.Error(file, err)
			}
			tracer.Done(file, dat)
			return nil
		})
	}
}

func processData(d prospect.Data, file string, between time.Duration) (prospect.Data, error) {
	if err := prospect.ReadFile(&d, file); err != nil {
		return d, err
	}
	return readFile(d, between)
}

func readFile(d prospect.Data, between time.Duration) (prospect.Data, error) {
	r, err := prospect.OpenFile(d.File)
	if err != nil {
		return d, err
	}

	rs := csv.NewReader(r)
	rs.Comma = getDelimiter(d.Mime)
	rs.ReuseRecord = true

	row, err := rs.Read()
	if err != nil {
		return d, err
	}
	for i := range row {
		d.Register(fmt.Sprintf(fileHeader, i+1), row[i])
	}

	var (
		count int
		runs  = make(map[string]int)
	)
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

		runs[row[1]]++
	}
	if count == 0 {
		return d, prospect.ErrIgnore
	}
	// d.Register(prospect.FileDuration, d.ModTime.Sub(d.AcqTime))
	d.Register(prospect.FileDuration, time.Duration(count)*between)
	d.Register(prospect.FileRecord, count)

	var i int
	for upi, count := range runs {
		i++
		d.Register(fmt.Sprintf(scienceRun, i), upi)
		d.Register(fmt.Sprintf(scienceRec, i), count)
		d.Register(fmt.Sprintf(scienceDur, i), time.Duration(count)*between)
	}
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
