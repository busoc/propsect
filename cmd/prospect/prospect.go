package main

import (
	"crypto/sha256"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/busoc/rt"
	"github.com/midbel/toml"
)

type Meta struct {
	Id   int    `xml:"-"`
	Accr string `toml:"acronym" xml:",comment"`

	Name       string    `toml:"experiment" xml:"experimentName"`
	Starts     time.Time `toml:"dtstart" xml:"startTime"`
	Ends       time.Time `toml:"dtend" xml:"endTime"`
	Domains    []string  `toml:"fields" xml:"researchField"`
	Increments []string  `xml:"increments>increment"`
	People     []string  `toml:"coordinators" xml:"scienceTeamCoordinators>coordinatorName,omitempty"`
	Payloads   []Payload `toml:"payload" xml:"payloads>payload"`
}

func (m *Meta) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	e.EncodeElement(m.Name, startElement("experimentName"))
	e.EncodeElement(strings.Join(m.Domains, ", "), startElement("researchField"))
	cs := struct {
		Values []string `xml:"coordinatorName"`
	}{
		Values: m.People,
	}
	e.EncodeElement(cs, startElement("scienceTeamCoordinators"))
	ps := struct {
		Values []Payload `xml:"payload"`
	}{
		Values: m.Payloads,
	}
	e.EncodeElement(ps, startElement("payloads"))
	e.EncodeElement(m.Starts.UTC(), startElement("startTime"))
	e.EncodeElement(m.Ends.UTC(), startElement("endTime"))
	is := struct {
		Values []string `xml:"increment"`
	}{
		Values: m.Increments,
	}
	e.EncodeElement(is, startElement("increments"))

	return nil
}

type Payload struct {
	XMLName xml.Name `toml:"-" xml:"payload"`
	Accr    string   `toml:"acronym" xml:"-"`
	Name    string   `xml:"payloadName"`
	Class   int      `xml:"payloadClass"`
}

type Mime struct {
	Extensions []string
	Type       string `toml:"type"`
}

func (m *Mime) Has(ext string) (string, bool) {
	if !sort.StringsAreSorted(m.Extensions) {
		sort.Strings(m.Extensions)
	}
	var (
		x    = sort.SearchStrings(m.Extensions, ext)
		ok   = x < len(m.Extensions) && m.Extensions[x] == ext
		mime string
	)
	if ok {
		mime = m.Type
	}
	return mime, ok
}

type Parameter struct {
	Name  string
	Value string
}

type Data struct {
	Experiment string `toml:"-"`
	Rootdir    string `toml:"rootdir"`
	File       string `toml:"datadir"`
	Level      int
	Source     string
	Integrity  string
	Type       string
	Model      string
	Mimes      []Mime `toml:"mimetype"`
	Crews      []string
	Owner      string
	Increments []string

	Path     string    `toml:"-"`
	Sum      string    `toml:"-"`
	AcqTime  time.Time `toml:"-"`
	ModTime  time.Time `toml:"-"`
	Mimetype string    `toml:"-"`

	Parameters []Parameter `toml:"-" xml:"experimentSpecificMetadata>parameter"`
}

func (d Data) MarshalXML(e *xml.Encoder, s xml.StartElement) error {
	e.EncodeElement(d.Experiment, startElement("experimentName"))
	e.EncodeElement(d.Model, startElement("model"))
	e.EncodeElement(d.Source, startElement("dataSource"))
	e.EncodeElement(d.Owner, startElement("dataOwner"))
	e.EncodeElement(d.AcqTime, startElement("acquisitionTime"))
	e.EncodeElement(d.ModTime, startElement("creationTime"))
	is := struct {
		Values []string `xml:"increment"`
	}{
		Values: d.Increments,
	}
	e.EncodeElement(is, startElement("increments"))
	cs := struct {
		Values []string `xml:"crewMemberName"`
	}{
		Values: d.Crews,
	}
	e.EncodeElement(cs, startElement("involvedCrew"))
	e.EncodeElement(d.Level, startElement("processingLevel"))
	e.EncodeElement(d.Type, startElement("productType"))
	e.EncodeElement(d.Mimetype, startElement("fileFormat"))
	e.EncodeElement(d.Path, startElement("relativePath"))
	xs := struct {
		Method string `xml:"method"`
		Value  string `xml:"value"`
	}{
		Method: d.Integrity,
		Value:  d.Sum,
	}
	e.EncodeElement(xs, startElement("integrity"))
	ps := struct {
		Values []Parameter `xml:"parameter"`
	}{
		Values: d.Parameters,
	}
	e.EncodeElement(ps, startElement("experimentSpecificMetadata"))

	return nil
}

func startElement(label string) xml.StartElement {
	n := xml.Name{Local: label}
	return xml.StartElement{Name: n}
}

func (d Data) guessType(ext string) string {
	mime := "application/octet-stream"
	for _, m := range d.Mimes {
		t, ok := m.Has(ext)
		if ok {
			mime = t
			break
		}
	}
	return mime
}

func main() {
	count := flag.Int("n", 0, "files")
	interval := flag.Bool("i", false, "interval")
	flag.Parse()
	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()

	d := struct {
		Rootdir string
		Meta
		Dataset []Data
	}{}
	if err := toml.NewDecoder(r).Decode(&d); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := marshalMeta(d.Rootdir, d.Meta); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var total int
	for i := range d.Dataset {
		for ds := range walkDataset(d.Dataset[i]) {
			if *interval && (ds.AcqTime.Before(d.Starts) || ds.AcqTime.After(d.Ends)) {
				continue
			}
			ds.Experiment = d.Meta.Name
			marshalData(d.Rootdir, ds)
			total++
			if *count > 0 && total >= *count {
				break
			}
		}
	}
}

func walkDataset(d Data) <-chan Data {
	queue := make(chan Data)

	go func() {
		defer close(queue)
		var (
			rootdir = d.File
			digest  = sha256.New()
			buf     = make([]byte, 8<<20)
		)
		filepath.Walk(rootdir, func(p string, i os.FileInfo, err error) error {
			defer digest.Reset()
			if err != nil {
				return err
			}
			if i.IsDir() {
				return nil
			}

			r, err := os.Open(p)
			if err != nil {
				return nil
			}
			defer r.Close()
			count, size, corrupted := readFile(io.TeeReader(rt.NewReader(r), digest), buf)

			x := d
			x.Parameters = x.Parameters[:0]

			x.File = filepath.Join(x.Rootdir, strings.TrimPrefix(p, rootdir))
			x.Path = x.File
			x.Mimetype = x.guessType(filepath.Ext(p))
			x.AcqTime = timeFromFile(p)
			x.ModTime = i.ModTime().UTC()
			x.Sum = fmt.Sprintf("%x", digest.Sum(nil))
			x.Parameters = append(x.Parameters, Parameter{Name: "file.num", Value: fmt.Sprintf("%d", count)})
			x.Parameters = append(x.Parameters, Parameter{Name: "file.size", Value: fmt.Sprintf("%d", size)})
			x.Parameters = append(x.Parameters, Parameter{Name: "file.corrupted", Value: fmt.Sprintf("%t", corrupted)})

			queue <- x
			return nil
		})
	}()
	return queue
}

func timeFromFile(file string) time.Time {
	var (
		parts = make([]int, 3)
		dir   = filepath.Dir(file)
		base  = filepath.Base(file)
		when  time.Time
	)
	for i := len(parts) - 1; i >= 0; i-- {
		d, f := filepath.Split(dir)
		x, err := strconv.Atoi(f)
		if err != nil {
			return when
		}
		parts[i] = x
		dir = filepath.Dir(d)
	}
	when = when.AddDate(parts[0]-1, 0, parts[1]).Add(time.Duration(parts[2]) * time.Hour)
	if x := strings.Index(base, "_"); x >= 0 {
		base = base[x+1:]
		if x = strings.Index(base, "_"); x >= 0 {
			x, err := strconv.Atoi(base[:x])
			if err == nil {
				when = when.Add(time.Duration(x) * time.Minute)
			}
		}
	}
	return when.UTC()
}

func readFile(rs io.Reader, buf []byte) (int64, int64, bool) {
	var size int64
	for i := 0; ; i++ {
		switch n, err := rs.Read(buf); err {
		case nil:
			size += int64(n)
		case io.EOF, rt.ErrInvalid:
			return int64(i), size, err == rt.ErrInvalid
		default:
			return 0, 0, true
		}
	}
}

func marshalData(dir string, d Data) error {
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

func marshalMeta(dir string, m Meta) error {
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

func encodeMeta(w io.Writer, m Meta) error {
	doc := struct {
		XMLName  xml.Name `xml:"http://eusoc.upm.es/SDC/Experiments/1 experiment"`
		Instance string   `xml:"xmlns:xsi,attr"`
		Location string   `xml:"xsi:schemaLocation,attr"`
		Meta     *Meta
	}{
		Meta:     &m,
		Instance: "http://www.w3.org/2001/XMLSchema-instance",
		Location: "experiment_metadata_schema.xsd",
	}
	return encodeDocument(w, doc)
}

func encodeData(w io.Writer, d Data) error {
	doc := struct {
		XMLName  xml.Name `xml:"http://eusoc.upm.es/SDC/Metadata/1 metadata"`
		Instance string   `xml:"xmlns:xsi,attr"`
		Location string   `xml:"xsi:schemaLocation,attr"`
		Data     *Data
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
