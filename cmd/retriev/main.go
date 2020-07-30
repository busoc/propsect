package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
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
		remote   = flag.String("r", "localhost:8090", "remote host (host:port)")
		instance = flag.String("i", "demo", "instance")
		user     = flag.String("u", "user", "username")
		passwd   = flag.String("p", "passwd", "password")
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

	base := filepath.Clean(flag.Arg(0))
	err := filepath.Walk(base, func(p string, i os.FileInfo, err error) error {
		if err != nil || i.IsDir() {
			return err
		}
		if filepath.Ext(p) != ".txt" {
			return nil
		}
		var (
			name = strings.TrimSuffix(strings.TrimPrefix(p, base), filepath.Ext(p))
			dir  = filepath.Join(flag.Arg(1), name)
		)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		return fetchData(p, dir, u, dtstart.Time.UTC(), dtend.Time.UTC())
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func fetchData(file, dir string, u url.URL, starts, ends time.Time) error {
	if ends.Before(starts) {
		return fmt.Errorf("invalid interval")
	}
	body, err := loadList(file)
	if err != nil {
		return err
	}
	for starts.Before(ends) {
		file := filepath.Join(dir, fmt.Sprintf("%04d", starts.Year()), fmt.Sprintf("%03d.tar", starts.YearDay()))
		if err := create(file, u, starts, body); err != nil {
			return err
		}
		starts = starts.Add(Day)
	}
	return nil
}

func create(file string, u url.URL, when time.Time, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	w, err := os.Create(file)
	if err != nil {
		return err
	}
	defer w.Close()
	log.Printf("create %s", file)

	tw := tar.NewWriter(w)
	defer tw.Close()

	retr := func(req *http.Request, when time.Time) error {
		rw, err := ioutil.TempFile("", "data*.csv.gz")
		if err != nil {
			return err
		}
		defer func() {
			rw.Close()
			os.Remove(rw.Name())
		}()

		if size, err := execute(rw, req); err != nil || size == 0 {
			return err
		}
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

	vs := url.Values{}
	for i := 0; i < 24; i++ {
		var (
			starts = when.Add(time.Duration(i) * time.Hour)
			ends   = starts.Add(time.Hour)
		)

		vs.Set("start", starts.Format(time.RFC3339))
		vs.Set("stop", ends.Format(time.RFC3339))
		u.RawQuery = vs.Encode()

		req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("accept", "text/csv")
		if err := retr(req, starts); err != nil {
			return err
		}
	}
	return nil
}

func execute(w io.Writer, req *http.Request) (int64, error) {
	z, _ := gzip.NewWriterLevel(w, gzip.BestCompression)
	defer z.Close()

	log.Printf("start query %s", req.URL.String())
	defer log.Printf("done query %s", req.URL.String())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return io.Copy(z, resp.Body)
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
