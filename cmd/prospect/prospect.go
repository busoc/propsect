package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/busoc/prospect"
	"github.com/midbel/toml"
	"golang.org/x/sync/errgroup"
)

func main() {
	flag.Parse()
	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()

	d := struct {
		Rootdir string
		prospect.Meta
		Dataset []prospect.Data
	}{}
	if err := toml.NewDecoder(r).Decode(&d); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for i := range d.Dataset {
		d.Dataset[i].Experiment = d.Meta.Name
		if err := processData(d.Dataset[i], d.Rootdir, d.Meta.Starts, d.Meta.Ends); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	if err := marshalMeta(d.Rootdir, d.Meta); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func processData(d prospect.Data, rootdir string, starts, ends time.Time) error {
	var group errgroup.Group
	for _, p := range d.Plugins {
		if p.Integrity == "" {
			p.Integrity = d.Integrity
		}
		if p.Type == "" {
			p.Type = d.Type
		}
		mod, err := p.Open()
		switch err {
		case prospect.ErrSkip:
			continue
		case nil:
		default:
			return err
		}
		group.Go(runModule(mod, d, rootdir, starts, ends))
	}
	return group.Wait()
}

func runModule(mod prospect.Module, d prospect.Data, rootdir string, starts, ends time.Time) func() error {
	return func() error {
		fmt.Println("start running module", mod.String())
		for {
			switch i, err := mod.Process(); err {
			case nil:
				if i.AcqTime.Before(starts) || i.AcqTime.After(ends) || i.File == "" {
					continue
				}
				i.Mime = d.GuessType(filepath.Ext(i.File))

				x := d
				x.Info = i

				file := filepath.Join(rootdir, d.Rootdir, i.File)
				if err := marshalData(file, x); err != nil {
					return fmt.Errorf("metadata: %s", err)
				}
			case prospect.ErrDone:
				return nil
			default:
				return fmt.Errorf("%s: %s", mod, err)
			}
		}
	}
}

func marshalData(dir string, d prospect.Data) error {
	file := filepath.Join(dir, d.File)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	w, err := os.Create(file + ".xml")
	if err != nil {
		return err
	}
	defer w.Close()
	return encodeData(w, d)
}

func marshalMeta(dir string, m prospect.Meta) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	w, err := os.Create(filepath.Join(dir, fmt.Sprintf("MD_EXP_%s.xml", m.Accr)))
	if err != nil {
		return err
	}
	defer w.Close()
	return encodeMeta(w, m)
}

func encodeMeta(w io.Writer, m prospect.Meta) error {
	doc := struct {
		XMLName  xml.Name `xml:"http://eusoc.upm.es/SDC/Experiments/1 experiment"`
		Instance string   `xml:"xmlns:xsi,attr"`
		Location string   `xml:"xsi:schemaLocation,attr"`
		Meta     *prospect.Meta
	}{
		Meta:     &m,
		Instance: "http://www.w3.org/2001/XMLSchema-instance",
		Location: "experiment_metadata_schema.xsd",
	}
	return encodeDocument(w, doc)
}

func encodeData(w io.Writer, d prospect.Data) error {
	doc := struct {
		XMLName  xml.Name `xml:"http://eusoc.upm.es/SDC/Metadata/1 metadata"`
		Instance string   `xml:"xmlns:xsi,attr"`
		Location string   `xml:"xsi:schemaLocation,attr"`
		Data     *prospect.Data
	}{
		Data:     &d,
		Instance: "http://www.w3.org/2001/XMLSchema-instance",
		Location: "file_metadata_schema.xsd",
	}
	return encodeDocument(w, doc)
}

func encodeDocument(w io.Writer, doc interface{}) error {
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	e := xml.NewEncoder(io.MultiWriter(os.Stdout, w))
	e.Indent("", "\t")
	return e.Encode(doc)
}
