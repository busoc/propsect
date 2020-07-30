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
)

const API = "/api/archive/%s/downloads/parameters/"
const Day = time.Hour * 24

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

// retr [-r remote] [-i instance] [-f from] [-t to] <listing dir> <archive dir>
func main() {
	var (
		dtstart  = NewDate(-7 * Day)
		dtend    = NewDate(1)
		secure   = flag.Bool("https", false, "https")
		remote   = flag.String("r", "localhost:8090", "remote host (host:port)")
		instance = flag.String("i", "demo", "instance")
		user     = flag.String("u", "user", "username")
		passwd   = flag.String("p", "passwd", "password")
		// archive  = flag.String("t", "", "archive type")
	)
	flag.Var(&dtstart, "f", "from date")
	flag.Var(&dtend, "t", "to date")
	flag.Parse()

	u := url.URL{
		Scheme: "http",
		Host:   *remote,
		Path:   fmt.Sprintf(API, *instance),
		User:   url.UserPassword(*user, *passwd),
	}
	if *secure {
		u.Scheme = "https"
	}

	base := filepath.Clean(flag.Arg(0))
	err := filepath.Walk(base, func(file string, i os.FileInfo, err error) error {
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
		dir, err := mkdir(base, flag.Arg(1), file)
		return fetchData(dir, u, body, dtstart.Time.UTC(), dtend.Time.UTC())
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func fetchData(dir string, api url.URL, body []byte, starts, ends time.Time) error {
	if ends.Before(starts) {
		return fmt.Errorf("invalid interval")
	}
	err := timeRange(starts, ends, Day, func(when time.Time) error {
		var (
			year = fmt.Sprintf("%04d", when.Year())
			doy  = fmt.Sprintf("%03d", when.YearDay())
			file = filepath.Join(dir, year, doy) + ".tar"
		)
		return create(file, api, when, body)
	})
	if errors.Is(err, io.EOF) {
		err = nil
	}
	return err
}

func create(file string, api url.URL, when time.Time, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	w, err := os.Create(file)
	if err != nil {
		return err
	}
	defer w.Close()
	log.Printf("begin writting %s", file)
	defer log.Printf("end writting %s", file)

	tw := tar.NewWriter(w)
	defer tw.Close()

	return timeRange(when, when.Add(Day), time.Hour, func(when time.Time) error {
		time.Sleep(time.Second)
		req, err := prepare(api, when, body)
		if err != nil {
			return err
		}
		rw, err := ioutil.TempFile("", "data*.csv.gz")
		if err != nil {
			return err
		}
		defer func() {
			rw.Close()
			os.Remove(rw.Name())
		}()

		if size, err := writeData(rw, req); err != nil || size == 0 {
			return err
		}
		h, err := makeHeader(rw, when)
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(&h); err != nil {
			return err
		}
		_, err = io.Copy(tw, rw)
		return err
	})
}

func prepare(api url.URL, when time.Time, body []byte) (*http.Request, error) {
	vs := url.Values{}
	vs.Set("start", when.Format(time.RFC3339))
	vs.Set("stop", when.Add(time.Hour).Format(time.RFC3339))
	api.RawQuery = vs.Encode()

	req, err := http.NewRequest(http.MethodPost, api.String(), bytes.NewReader(body))
	if err == nil {
		req.Header.Set("accept", "text/csv")
	}
	return req, err
}

func writeData(ws io.WriteSeeker, req *http.Request) (int64, error) {
	z, _ := gzip.NewWriterLevel(ws, gzip.BestCompression)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	size, err := io.Copy(z, resp.Body)
	if err != nil {
		return size, err
	}
	if err := z.Close(); err != nil {
		return size, err
	}
	_, err = ws.Seek(0, io.SeekStart)
	return size, err
}

func timeRange(starts, ends time.Time, step time.Duration, fn TimeFunc) error {
	if starts.After(ends) {
		return fmt.Errorf("invalid interval: %s - %s", starts, ends)
	}
	var err error
	for starts.Before(ends) {
		if err = fn(starts); err != nil {
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

func makeHeader(rw *os.File, when time.Time) (tar.Header, error) {
	z, err := rw.Stat()
	if err != nil {
		return tar.Header{}, err
	}
	hdr := tar.Header{
		Name:    fmt.Sprintf("%02d.csv.gz", when.Hour()),
		Size:    z.Size(),
		ModTime: when.UTC(),
		Uid:     1000,
		Gid:     1000,
		Mode:    0644,
	}
	return hdr, nil
}
