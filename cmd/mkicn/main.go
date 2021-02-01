package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/busoc/prospect"
	"github.com/busoc/prospect/cmd/internal/trace"
	"github.com/midbel/mime"
)

const (
	titlePattern = `[a-zA-Z ]+?\(?(GMT ?\d{3}\/\d{2}:\d{2})\)?[a-zA-Z ]?\(?(GMT ?\d{3}\/\d{2}:\d{2})?\)?`
	tablePattern = `(S_\d+)_([a-zA-Z0-9_]+?_FILE_\d{3}(_DAT)?(_File)?)_(.+?)(?:_?[vV]\d+?)??_(\d{2}_\d{3}_\d{2}_\d{2}).*`
)

var (
	titleExp = regexp.MustCompile(titlePattern)
	tableExp = regexp.MustCompile(tablePattern)
)

type List struct {
	Records []string
}

func (i *List) Set(str string) error {
	r, err := os.Open(str)
	if err != nil {
		return err
	}
	defer r.Close()

	i.Records = readList(r)
	return nil
}

func (i *List) String() string {
	return "list of records"
}

func main() {
	var list List
	flag.Var(&list, "list", "list of filename to keep")
	flag.Parse()

	accept := func(d prospect.Data) bool {
		if d.Type == prospect.TypeICN {
			return true
		}
		mt, err := mime.Parse(d.Mime)
		if err != nil {
			return false
		}
		return strings.ToLower(mt.Params["type"]) == "icn"
	}
	err := prospect.Build(flag.Arg(0), collectData(list.Records), accept)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func collectData(list []string) prospect.RunFunc {
	return func(b prospect.Builder, d prospect.Data) {
		tracer := trace.New("mkicn")
		defer tracer.Summarize()

		filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
			if err != nil || i.IsDir() || !d.Accept(file) {
				return err
			}
			tracer.Start(file)
			defer tracer.Done(file, dat)

			dat, files, err := processConsoleNote(d.Clone(), file, list)
			if err != nil {
				tracer.Error(file, err)
				return nil
			}
			links := storeTables(b, files, prospect.CreateLinkFrom(dat))
			if len(links) == 0 {
				return nil
			}
			dat.Links = append(dat.Links, links...)
			if err := b.Store(dat); err != nil {
				tracer.Error(file, err)
			}
			return nil
		})
	}
}

func storeTables(b prospect.Builder, files []prospect.Data, link prospect.Link) []prospect.Link {
	var (
		links  []prospect.Link
		tracer = trace.New("mkicn")
	)
	defer tracer.Summarize()

	for _, f := range files {
		tracer.Start(f.File)
		f, err := processTable(f)
		if err != nil {
			tracer.Error(f.File, err)
			continue
		}
		f.Links = append(f.Links, link)
		if err := b.Store(f); err != nil {
			tracer.Error(f.File, err)
			continue
		}
		links = append(links, prospect.CreateLinkFrom(f))
		tracer.Done(f.File, f)
	}
	return links
}

const (
	Filename = "Filename:"
	Title    = "Title:"
)

func processTable(d prospect.Data) (prospect.Data, error) {
	for _, e := range []string{"", ".DAT"} {
		i, err := os.Stat(d.File + e)
		if err == nil && i.Mode().IsRegular() {
			d.File = d.File + e
			break
		}
	}
	return d, prospect.ReadFile(&d, d.File)
}

func processConsoleNote(d prospect.Data, file string, list []string) (prospect.Data, []prospect.Data, error) {
	if err := prospect.ReadFile(&d, file); err != nil {
		return d, nil, err
	}
	files, err := readConsoleNote(&d, list)
	return d, files, err
}

func readConsoleNote(d *prospect.Data, list []string) ([]prospect.Data, error) {
	r, err := prospect.OpenFile(d.File)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var (
		when = parseFilename(d.File)
		scan = bufio.NewScanner(r)
	)
	if !scan.Scan() {
		return nil, nil
	}
	d.AcqTime, d.ModTime = parseTitle(scan.Text(), when)

	var files []prospect.Data
	for scan.Scan() {
		line := scan.Text()
		if !strings.HasPrefix(line, Filename) {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, Filename))
		if ok, upi, when := keepFile(line, list); ok {
			x := d.Clone()

			x.AcqTime = when
			x.ModTime = when
			x.File = filepath.Join(filepath.Dir(d.File), line)
			x.Type = prospect.TypeParamTable
			x.Mime = prospect.MimePlain
			x.Register(prospect.ScienceRun, upi)

			files = append(files, x)
		}
	}
	return files, nil
}

func keepFile(file string, list []string) (bool, string, time.Time) {
	var (
		when time.Time
		ms   = tableExp.FindAllStringSubmatch(file, -1)
	)
	if len(ms) == 0 {
		return false, "", when
	}
	var (
		n  = len(ms[0]) - 2
		ps = ms[0][n:]
		ix = sort.SearchStrings(list, ps[0])
	)
	if len(list) > 0 && (ix >= len(list) || list[ix] != ps[0]) {
		return false, "", when
	}
	when, _ = time.Parse("2006_002_15_04", "20"+ps[1])
	return true, ps[0], when
}

func parseTitle(str string, base time.Time) (time.Time, time.Time) {
	var upk, xfer time.Time
	str = strings.Trim(str, "\xef\xbb\xbf")
	if str == "" || !strings.HasPrefix(str, Title) {
		return base, base
	}
	str = strings.TrimSpace(strings.TrimPrefix(str, Title))

	ms := titleExp.FindAllStringSubmatch(str, 2)
	if len(ms) == 0 {
		return base, base
	} else if len(ms) == 1 {
		ms = append(ms, ms[0])
	}
	upk, _ = time.Parse("GMT002/15:04", strings.ReplaceAll(ms[0][1], " ", ""))
	xfer, _ = time.Parse("GMT002/15:04", strings.ReplaceAll(ms[1][1], " ", ""))

	if upk.Year() == 0 {
		upk = upk.AddDate(base.Year(), 0, 0)
	}
	if xfer.Year() == 0 {
		xfer = xfer.AddDate(base.Year(), 0, 0)
	}
	return upk, xfer
}

func parseFilename(file string) time.Time {
	var (
		dir  = filepath.Clean(filepath.Dir(file))
		doy  = filepath.Base(dir)
		year = filepath.Base(filepath.Dir(dir))
	)

	if !strings.HasPrefix(doy, "GMT") {
		doy = year
		year = filepath.Base(filepath.Dir(filepath.Dir(dir)))
	}
	if len(doy) > 6 {
		doy = doy[:6]
	}

	when, _ := time.Parse("GMT002/2006", fmt.Sprintf("%s/%s", doy, year))
	return when
}

func readList(r io.Reader) []string {
	var (
		list []string
		seen = make(map[string]struct{})
		scan = bufio.NewScanner(r)
	)
	for scan.Scan() {
		line := scan.Text()
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		list = append(list, strings.TrimSpace(line))
	}
	sort.Strings(list)
	return list
}
