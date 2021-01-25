package main

import (
	"crypto/md5"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/busoc/prospect"
	"github.com/midbel/toml"
)

type Mime struct {
	Extensions []string
	Mime       string
	Type       string
}

type Data struct {
	prospect.Data
	Mimes   []Mime
	Archive string
}

func (d *Data) Process() error {
	r, err := os.Open(d.File)
	if err != nil {
		return err
	}
	defer r.Close()

	var (
		sumSHA = sha256.New()
		sumMD5 = md5.New()
	)
	if d.Size, err = io.Copy(io.MultiWriter(sumSHA, sumMD5), r); err != nil {
		return err
	}

	d.Integrity = "SHA256"
	d.Sum = fmt.Sprintf("%x", sumSHA.Sum(nil))
	d.MD5 = fmt.Sprintf("%x", sumMD5.Sum(nil))

	return nil
}

func main() {
	flag.Parse()

	c := struct {
		prospect.Archive
		prospect.Context
		Data []Data `toml:"file"`
	}{}
	if err := toml.DecodeFile(flag.Arg(0), &c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, d := range c.Data {
		if i, err := os.Stat(d.File); err != nil || !i.Mode().IsRegular() {
			continue
		}
		log.Printf("start processing %s", d.File)
		now := time.Now()
		d.Data = c.Update(d.Data)
		if err := d.Process(); err != nil {
			log.Printf("fail to update %s: %s", d.File, err)
			continue
		}
		if err := c.Store(d.Data, d.Archive); err != nil {
			log.Printf("fail to store %s: %s", d.File, err)
			continue
		}
		log.Printf("done %s (%d - %s - %s)", d.File, d.Size, time.Since(now), d.MD5)
	}
}
