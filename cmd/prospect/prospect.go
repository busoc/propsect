package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/busoc/prospect"
	"github.com/midbel/toml"
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
	fmt.Printf("%+v\n", d)
	for i := range d.Dataset {
		if err := processData(d.Dataset[i]); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	if err := marshalMeta(d.Rootdir, d.Meta); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	// for i := range d.Dataset {
	// 	for ds := range walkDataset(d.Dataset[i]) {
	// 		ds.Experiment = d.Meta.Name
	// 		marshalData(d.Rootdir, ds)
	// 	}
	// }
}

func processData(d prospect.Data) error {
	for _, p := range d.Plugins {
		if p.Integrity == "" {
			p.Integrity = d.Integrity
		}
		_, err := p.Open()
		switch err {
		case prospect.ErrSkip:
			continue
		case nil:
		default:
			return err
		}
	}
	return nil
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