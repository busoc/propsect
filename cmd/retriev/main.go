package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/midbel/try"
)

const (
	APIP = "/api/archive/%s/downloads/parameters/"
	APIE = "/api/archive/%s/downloads/events/"
	APIC = "/api/archive/%s/commands"
)
const (
	Day = time.Hour * 24
	MaxAttempt = 10
)

// retr [-r remote] [-i instance] [-f from] [-t to] <listing dir> <archive dir>
func main() {
	var (
		dtstart  = NewDate(-7 * Day)
		dtend    = NewDate(1)
		secure   = flag.Bool("https", false, "https")
		flat     = flag.Bool("flat", false, "put data in flat files instead of tar")
		remote   = flag.String("r", "localhost:8090", "remote host (host:port)")
		instance = flag.String("i", "demo", "instance")
		user     = flag.String("u", "user", "username")
		passwd   = flag.String("p", "passwd", "password")
		minify   = flag.Bool("c", false, "compress downloaded files")
		archive  = flag.String("a", "", "archive type")
	)
	flag.Var(&dtstart, "f", "from date")
	flag.Var(&dtend, "t", "to date")
	flag.Parse()

	u := url.URL{
		Scheme: "http",
		Host:   *remote,
		User:   url.UserPassword(*user, *passwd),
	}
	if *secure {
		u.Scheme = "https"
	}
	var err error
	switch strings.ToLower(*archive) {
	case "", "parameters":
		u.Path = fmt.Sprintf(APIP, *instance)
		err = retrParameters(u, *minify, *flat, dtstart.Time.UTC(), dtend.Time.UTC(), flag.Arg(0), flag.Arg(1))
	case "events":
		u.Path = fmt.Sprintf(APIE, *instance)
		err = retrEvents(u, *minify, *flat, dtstart.Time.UTC(), dtend.Time.UTC(), flag.Arg(0))
	case "commands":
		u.Path = fmt.Sprintf(APIC, *instance)
		err = retrCommands(u, *minify, *flat, dtstart.Time.UTC(), dtend.Time.UTC(), flag.Arg(0))
	default:
		err = fmt.Errorf("%s: unknown archive type", *archive)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

type TimeFunc func(time.Time) error

type Date struct {
	time.Time
}

func NewDate(delta time.Duration) Date {
	t := time.Now().UTC()
	t = t.Add(delta).Truncate(Day)
	return Date{Time: t}
}

func (d *Date) Set(str string) error {
	w, err := time.Parse("2006-01-02", str)
	if err == nil {
		d.Time = w.UTC()
	}
	return err
}

func (d *Date) String() string {
	if d.IsZero() {
		return "yyyy-mm-dd"
	}
	return d.Format("2006-01-02")
}

const (
	fmtText uint8 = iota
	fmtCsv
	fmtJson
)

type Request struct {
	api    url.URL
	body   []byte
	format uint8
	mini   bool
}

func TextRequest(api url.URL, body []byte, minify bool) Request {
	return Request{
		api:    api,
		body:   body,
		format: fmtText,
		mini:   minify,
	}
}

func CsvRequest(api url.URL, body []byte, minify bool) Request {
	return Request{
		api:    api,
		body:   body,
		format: fmtCsv,
		mini:   minify,
	}
}

func JsonRequest(api url.URL, body []byte, minify bool) Request {
	return Request{
		api:    api,
		body:   body,
		format: fmtJson,
		mini:   minify,
	}
}

func (r Request) Ext() string {
	var e string
	switch r.format {
	case fmtJson:
		e = ".json"
	case fmtText:
		e = ".txt"
	case fmtCsv:
		e = ".csv"
	default:
		e = ".dat"
	}
	if r.mini {
		e += ".gz"
	}
	return e
}

func (r Request) Copy(ws io.Writer, starts, ends time.Time) (int64, error) {
	req, err := r.Make(starts, ends)
	if err != nil {
		return 0, err
	}
	if r.mini {
		z, _ := gzip.NewWriterLevel(ws, gzip.BestCompression)
		defer z.Close()
		ws = z
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return io.Copy(ws, resp.Body)
}

func (r Request) Make(starts, ends time.Time) (*http.Request, error) {
	vs := url.Values{}
	vs.Set("start", starts.Format(time.RFC3339))
	vs.Set("stop", ends.Format(time.RFC3339))
	r.api.RawQuery = vs.Encode()

	method := http.MethodGet
	if len(r.body) > 0 {
		method = http.MethodPost
	}

	req, err := http.NewRequest(method, r.api.String(), bytes.NewReader(r.body))
	if err == nil {
		req.Header.Set("accept-encoding", "identity")
		if r.isText() {
			req.Header.Set("accept", "text/csv")
		}
	}
	return req, err
}

func (r Request) isText() bool {
	return r.format == fmtText || r.format == fmtCsv
}

func (r Request) String() string {
	return r.api.String()
}

func retrParameters(api url.URL, mini, flat bool, dtstart, dtend time.Time, base, dir string) error {
	base = filepath.Clean(base)
	return filepath.Walk(base, func(file string, i os.FileInfo, err error) error {
		if err != nil || i.IsDir() {
			return err
		}
		if filepath.Ext(file) != ".txt" {
			return nil
		}
		body, err := loadList(file)
		if err != nil {
			return err
		}
		dst, err := mkdir(base, dir, file)
		if err == nil {
			req := CsvRequest(api, body, mini)
			err = fetchData(dst, flat, req, dtstart, dtend)
		}
		return err
	})
}

func retrCommands(api url.URL, mini, flat bool, dtstart, dtend time.Time, dir string) error {
	return fetchData(dir, flat, JsonRequest(api, nil, mini), dtstart, dtend)
}

func retrEvents(api url.URL, mini, flat bool, dtstart, dtend time.Time, dir string) error {
	return fetchData(dir, flat, TextRequest(api, nil, mini), dtstart, dtend)
}

func fetchData(dir string, flat bool, req Request, starts, ends time.Time) error {
	if ends.Before(starts) {
		return fmt.Errorf("invalid interval")
	}
	return timeRange(starts, ends, Day, func(when time.Time) error {
		var (
			year = fmt.Sprintf("%04d", when.Year())
			doy  = fmt.Sprintf("%03d", when.YearDay())
			file = filepath.Join(dir, year, doy)
		)
		if flat {
			return createFile(file+req.Ext(), req, when, when.Add(Day))
		}
		return createArchive(file+".tar", req, when)
	})
}

func createFile(file string, req Request, starts, ends time.Time) error {
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	log.Printf("begin writting %s", file)
	defer log.Printf("end writting %s", file)

	return try.Try(MaxAttempt, func(i int) error {
		w, err := os.Create(file)
		if err != nil {
			return err
		}
		defer w.Close()

		_, err = req.Copy(w, starts, ends)
		if err != nil {
			log.Printf("%s: attempt #%d failed: %v", req, i, err)
		}
		return err
	})
}

func createArchive(file string, req Request, when time.Time) error {
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	w, err := os.Create(file)
	if err != nil {
		return err
	}
	log.Printf("begin writting %s", file)
	defer log.Printf("end writting %s", file)

	var (
		written int
		tw      = tar.NewWriter(w)
	)
	defer func() {
		tw.Close()
		w.Close()

		if written == 0 {
			os.Remove(file)
		}
	}()

	return timeRange(when, when.Add(Day), time.Hour, func(when time.Time) error {
		return try.Try(MaxAttempt, func(i int) error {
			rw, err := ioutil.TempFile("", "data*.csv.gz")
			if err != nil {
				return err
			}
			defer func() {
				rw.Close()
				os.Remove(rw.Name())
			}()

			if size, err := req.Copy(rw, when, when.Add(time.Hour)); err != nil || size == 0 {
				if err != nil {
					log.Printf("%s: attempt #%d failed: %v", req, i, err)
				}
				return err
			}
			written++
			if err := appendFile(tw, rw, when); err != nil {
				return fmt.Errorf("%w: %s", wip.ErrAbort, err)
			}
			return nil
		})
	})
}

func appendFile(tw *tar.Writer, rw *os.File, when time.Time) error {
	if _, err := rw.Seek(0, io.SeekStart); err != nil {
		return err
	}
	z, err := rw.Stat()
	if err != nil {
		return err
	}
	h := tar.Header{
		Name:    fmt.Sprintf("%02d.csv.gz", when.Hour()),
		Size:    z.Size(),
		ModTime: when.UTC(),
		Uid:     1000,
		Gid:     1000,
		Mode:    0644,
	}
	if err := tw.WriteHeader(&h); err != nil {
		return err
	}
	_, err = io.Copy(tw, rw)
	return err
}

func timeRange(starts, ends time.Time, step time.Duration, fn TimeFunc) error {
	if starts.After(ends) {
		return fmt.Errorf("invalid interval: %s - %s", starts, ends)
	}
	var err error
	for starts.Before(ends) {
		if err = fn(starts); err != nil && !errors.Is(err, io.EOF) {
			break
		}
		starts = starts.Add(step)
	}
	return err
}

func loadList(file string) ([]byte, error) {
	type pair struct {
		Name  string `json:"name"`
		Space string `json:"namespace,omitempty"`
	}

	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var (
		ds []pair
		sc = bufio.NewScanner(r)
	)
	for sc.Scan() {
		line := sc.Text()
		if line[0] != '-' {
			continue
		}
		p := pair{
			Name: strings.TrimSpace(line[1:]),
		}
		ds = append(ds, p)
	}

	datum := struct {
		Id []pair `json:"id"`
	}{
		Id: ds,
	}
	return json.Marshal(datum)
}

func mkdir(base, dir, file string) (string, error) {
	name := strings.TrimSuffix(strings.TrimPrefix(file, base), filepath.Ext(file))
	dir = filepath.Join(dir, name)
	return dir, os.MkdirAll(dir, 0755)
}
