package prospect

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/xml"
	"errors"
	"fmt"
	"hash"
	"plugin"
	"sort"
	"strings"
	"time"
)

var (
	ErrSkip = errors.New("skip module")
	ErrDone = errors.New("done")
)

type FileInfo struct {
	File string
	Type string
	Mime string

	Integrity string
	Sum       string
	Size      int

	ModTime time.Time
	AcqTime time.Time

	Parameters []Parameter
}

type Module interface {
	Process() (FileInfo, error)
}

type Config struct {
	Integrity string
	Module    string
	Location  string
	Type      string
}

func (c Config) Hash() hash.Hash {
	var h hash.Hash
	switch strings.ToLower(c.Integrity) {
	case "sha256", "sha-256":
		h = sha256.New()
	case "sha512", "sha-512":
		h = sha512.New512_256()
	case "md5":
		h = md5.New()
	default:
		h = sha1.New()
	}
	return h
}

func (c Config) Open() (Module, error) {
	if c.Module == "" {
		return nil, ErrSkip
	}
	g, err := plugin.Open(c.Module)
	if err != nil {
		return nil, err
	}
	sym, err := g.Lookup("New")
	if err != nil {
		return nil, err
	}
	fn, ok := sym.(func(Config) Module)
	if !ok {
		return nil, fmt.Errorf("%s: invalid plugin - invalid signture (%T)", c.Module, sym)
	}
	return fn(c), nil
}

type Parameter struct {
	Name  string
	Value string
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

type Meta struct {
	Id   int    `xml:"-"`
	Accr string `toml:"acronym" xml:",comment"`

	Name       string    `toml:"experiment"`
	Starts     time.Time `toml:"dtstart"`
	Ends       time.Time `toml:"dtend"`
	Domains    []string  `toml:"fields"`
	Increments []string
	People     []string  `toml:"coordinators"`
	Payloads   []Payload `toml:"payload"`
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

type Data struct {
	Experiment string `toml:"-"`
	File       string `toml:"-"`
	Rootdir    string `toml:"rootdir"`
	Level      int
	Source     string
	Integrity  string
	Type       string
	Model      string
	Mimes      []Mime `toml:"mimetype"`
	Crews      []string
	Owner      string
	Increments []string
	Plugins    []Config `toml:"data"`

	Info FileInfo
}

func (d Data) MarshalXML(e *xml.Encoder, s xml.StartElement) error {
	e.EncodeElement(d.Experiment, startElement("experimentName"))
	e.EncodeElement(d.Model, startElement("model"))
	e.EncodeElement(d.Source, startElement("dataSource"))
	e.EncodeElement(d.Owner, startElement("dataOwner"))
	e.EncodeElement(d.Info.AcqTime, startElement("acquisitionTime"))
	e.EncodeElement(d.Info.ModTime, startElement("creationTime"))
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
	if d.Info.Type == "" {
		d.Info.Type = d.Type
	}
	e.EncodeElement(d.Info.Type, startElement("productType"))
	e.EncodeElement(d.Info.Mime, startElement("fileFormat"))
	e.EncodeElement(d.Info.File, startElement("relativePath"))
	xs := struct {
		Method string `xml:"method"`
		Value  string `xml:"value"`
	}{
		Method: d.Info.Integrity,
		Value:  d.Info.Sum,
	}
	e.EncodeElement(xs, startElement("integrity"))
	ps := struct {
		Values []Parameter `xml:"parameter"`
	}{
		Values: d.Info.Parameters,
	}
	e.EncodeElement(ps, startElement("experimentSpecificMetadata"))

	return nil
}

func (d Data) GuessType(ext string) string {
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

func startElement(label string) xml.StartElement {
	n := xml.Name{Local: label}
	return xml.StartElement{Name: n}
}
