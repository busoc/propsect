package main

import (
  "errors"
	"crypto/md5"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
  "log"
	"os"
	"path/filepath"
	"sort"
  "time"

	"github.com/busoc/prospect"
	"github.com/midbel/toml"
)

type Data struct {
	prospect.Data
	Archive string
}

func (d *Data) Update() error {
	r, err := os.Open(d.File)
	if err != nil {
		return err
	}
	defer r.Close()

	var (
		sumSHA = sha256.New()
		sumMD5 = md5.New()
		ws     = io.MultiWriter(sumSHA, sumMD5)
	)
	if d.Size, err = io.Copy(ws, r); err != nil {
		return err
	}

	d.Integrity = "SHA256"
	d.Sum = fmt.Sprintf("%x", sumSHA.Sum(nil))
	d.MD5 = fmt.Sprintf("%x", sumMD5.Sum(nil))

	return nil
}

type Archive struct {
	DataDir string `toml:"datadir"`
	MetaDir string `toml:"metadir"`
}

func (a Archive) Store(d Data) error {
	r, err := prospect.ParseResolver(d.Archive)
	if err != nil {
		return err
	}
	file := filepath.Join(r.Resolve(d.Data), filepath.Base(d.File))
	if err := a.storeLink(d, file); err != nil {
		return err
	}
	return a.storeMeta(d, file)
}

func (a Archive) storeMeta(d Data, file string) error {
	d.File = file
	file = filepath.Join(a.MetaDir, file)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	w, err := os.Create(file + ".xml")
	if err != nil {
		return err
	}
	defer w.Close()
	return prospect.EncodeData(w, d.Data)
}

func (a Archive) storeLink(d Data, file string) error {
	file = filepath.Join(a.DataDir, file)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	err := os.Link(d.File, file)
  if errors.Is(err, os.ErrExist) {
    err = nil
  }
	return err
}

type Default struct {
	Experiment string
	Model      string
	Source     string
	Owner      string

	Increments []prospect.Increment `toml:"increment"`
	Metadata   []prospect.Parameter
}

func (df Default) Update(d Data) Data {
	if d.Experiment == "" {
		d.Experiment = df.Experiment
	}
	if d.Source == "" {
		d.Source = df.Source
	}
	if d.Model == "" {
		d.Model = df.Model
	}
	if d.Owner == "" {
		d.Owner = df.Owner
	}
	if len(d.Increments) == 0 && len(df.Increments) > 0 {
		x := sort.Search(len(df.Increments), func(i int) bool {
			return df.Increments[i].Contains(d.AcqTime)
		})
		if x < len(df.Increments) && df.Increments[x].Contains(d.AcqTime) {
			d.Increments = append(d.Increments, df.Increments[x].Num)
		}
	}
	d.Parameters = append(d.Parameters, df.Metadata...)
	return d
}

func main() {
	flag.Parse()

	c := struct {
		Archive
		Default
		Data []Data `toml:"file"`
	}{}
	if err := toml.DecodeFile(flag.Arg(0), &c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, d := range c.Data {
    log.Printf("start processing %s", d.File)
    now := time.Now()
		d = c.Default.Update(d)
		if err := d.Update(); err != nil {
			log.Printf("update %s: %s", d.File, err)
			continue
		}
		if err := c.Store(d); err != nil {
			log.Printf("storing %s: %s", d.File, err)
      continue
		}
    log.Printf("done %s (%d - %s - %x)", d.File, d.Size, time.Since(now), d.Sum)
	}
}
