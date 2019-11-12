package prospect

import (
	"encoding/xml"
	"errors"
	"io"
	// "path/filepath"
	"sort"
	"strings"
	"time"
)

const DefaultMime = "application/octet-stream"

var (
	ErrSkip = errors.New("skip module")
	ErrDone = errors.New("done")
)

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

type Activity struct {
	Type   string `toml:"source"`
	Name   string
	Starts time.Time `toml:"dtstart"`
	Ends   time.Time `toml:"dtend"`
}

type Data struct {
	Experiment string `toml:"-"`
	Rootdir    string
	Level      int
	Source     string
	Integrity  string
	Type       string
	Model      string
	Crews      []string
	Owner      string
	Increments []string

	Info FileInfo
}

func (d Data) MarshalXML(e *xml.Encoder, s xml.StartElement) error {
	e.EncodeElement(d.Experiment, startElement("experimentName"))
	e.EncodeElement(d.Model, startElement("model"))
	e.EncodeElement(d.Source, startElement("dataSource"))
	e.EncodeElement(d.Owner, startElement("dataOwner"))
	e.EncodeElement(d.Info.AcqTime.Format(time.RFC3339), startElement("acquisitionTime"))
	e.EncodeElement(d.Info.ModTime.Format(time.RFC3339), startElement("creationTime"))
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

func startElement(label string) xml.StartElement {
	n := xml.Name{Local: label}
	return xml.StartElement{Name: n}
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
	e := xml.NewEncoder(w)
	e.Indent("", "\t")
	return e.Encode(doc)
}
