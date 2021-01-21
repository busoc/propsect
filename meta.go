package prospect

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

const (
	ptrRef = "ptr.%d.href"
	ptrRole = "ptr.%d.role"
	fileSize = "file.size"
	fileMD5 = "file.md5"
)

type Increment struct {
	Starts time.Time
	Ends   time.Time
	Num    string `toml:"increment"`
}

func (i Increment) Contains(t time.Time) bool {
	return i.Starts.Before(t) && i.Ends.After(t)
}

type Parameter struct {
	Name  string `xml:"name"`
	Value string `xml:"value"`
}

func MakeParameter(k string, v interface{}) Parameter {
	p := Parameter{
		Name:  k,
		Value: fmt.Sprintf("%v", v),
	}
	return p
}

type Payload struct {
	XMLName xml.Name `toml:"-" xml:"payload"`
	Accr    string   `toml:"acronym" xml:"-"`
	Name    string   `xml:"payloadName"`
	Class   int      `xml:"payloadClass"`
}

type Meta struct {
	Id   int
	Accr string `toml:"acronym" xml:",comment"`

	Name       string    `toml:"experiment"`
	Starts     time.Time `toml:"dtstart"`
	Ends       time.Time `toml:"dtend"`
	Domains    []string  `toml:"fields"`
	Increments []string
	People     []string  `toml:"coordinators"`
	Payloads   []Payload `toml:"payload"`
}

func (m Meta) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
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

type Link struct {
	File string
	Role string
}

type Data struct {
	Experiment string
	Level      int
	Source     string // Science Run, EST,...
	Integrity  string `toml:"-"`
	Sum        string `toml:"-"`
	Type       string // Doc, Image,...
	Model      string // FM, EM,...
	Crews      []string
	Owner      string
	Increments []string
	Mime       string
	File       string
	ModTime    time.Time
	AcqTime    time.Time

	Parameters []Parameter `toml:"metadata"`
	Links      []Link      `toml:"links"`

	Size int64
	MD5  string
}

func (d Data) MarshalXML(e *xml.Encoder, s xml.StartElement) error {
	e.EncodeElement(d.Experiment, startElement("experimentName"))
	e.EncodeElement(d.Model, startElement("model"))
	e.EncodeElement(d.Source, startElement("dataSource"))
	e.EncodeElement(d.Owner, startElement("dataOwner"))
	e.EncodeElement(d.AcqTime.Format(time.RFC3339), startElement("acquisitionTime"))
	e.EncodeElement(d.ModTime.Format(time.RFC3339), startElement("creationTime"))
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
	e.EncodeElement(d.Mime, startElement("fileFormat"))
	e.EncodeElement(d.File, startElement("relativePath"))
	xs := struct {
		Method string `xml:"method"`
		Value  string `xml:"value"`
	}{
		Method: d.Integrity,
		Value:  d.Sum,
	}
	e.EncodeElement(xs, startElement("integrity"))
	for i, k := range d.Links {
		var (
			h = MakeParameter(fmt.Sprintf(ptrRef, i+1), k.File)
			r = MakeParameter(fmt.Sprintf(ptrRole, i+1), k.Role)
		)
		d.Parameters = append(d.Parameters, h, r)
	}
	if d.Size > 0 {
		d.Parameters = append(d.Parameters, MakeParameter(fileSize, d.Size))
	}
	if d.MD5 != "" {
		d.Parameters = append(d.Parameters, MakeParameter(fileMD5, d.MD5))
	}
	sort.Slice(d.Parameters, func(i, j int) bool {
		return d.Parameters[i].Name < d.Parameters[j].Name
	})
	ps := struct {
		Values []Parameter `xml:"parameter"`
	}{
		Values: d.Parameters,
	}
	e.EncodeElement(ps, startElement("experimentSpecificMetadata"))

	return nil
}

func EncodeMeta(w io.Writer, m Meta) error {
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

func EncodeData(w io.Writer, d Data) error {
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

func startElement(label string) xml.StartElement {
	n := xml.Name{Local: label}
	return xml.StartElement{Name: n}
}
