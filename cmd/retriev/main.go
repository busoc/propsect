package main

import (
	"bufio"
	"bytes"
	// "compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
	_ = u

	base := filepath.Clean(flag.Arg(0))
	err := filepath.Walk(base, func(p string, i os.FileInfo, err error) error {
		if err != nil || i.IsDir() {
			return err
		}
		if filepath.Ext(p) == ".txt" {
			var (
				name = strings.TrimSuffix(strings.TrimPrefix(p, base), filepath.Ext(p))
				dir  = filepath.Join(flag.Arg(1), name)
			)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
			err = fetchData(p, dir, u, dtstart.Time, dtend.Time)
		}
		return err
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
	vs := url.Values{}
	for starts.Before(ends) {
		vs.Set("start", starts.Format(time.RFC3339))
		vs.Set("stop", starts.Add(Day).Format(time.RFC3339))
		u.RawQuery = vs.Encode()

		req, err := http.NewRequest(http.MethodPost, u.String(), body)
		if err != nil {
			return err
		}
		req.Header.Set("accept", "text/csv")
		file := filepath.Join(dir, fmt.Sprintf("%04d", starts.Year()), fmt.Sprintf("%03d.csv", starts.YearDay()))
		if err := execute(file, req); err != nil {
			return err
		}
		starts = starts.Add(Day)
		time.Sleep(5 * time.Second)
	}
	return nil
}

func execute(file string, req *http.Request) error {
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	w, err := os.Create(file)
	if err != nil {
		return err
	}
	defer w.Close()

	// z, _ := gzip.NewWriterLevel(w, gzip.BestCompression)
	// defer z.Close()

	fmt.Println("start query", req.URL.String())
	defer fmt.Println("done query", req.URL.String())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(w, resp.Body)
	return err
}

func loadList(file string) (io.Reader, error) {
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

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(datum); err != nil {
		return nil, err
	}
	return &buf, nil
}
