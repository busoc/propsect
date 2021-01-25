package prospect

import (
	"compress/gzip"
	"crypto/md5"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	SHA = "SHA256"

	ExtGZ = ".gz"

	MimePlain = "text/plain"
	MimeOctet = "application/octet-stream"
	MimeQuick = "video/quicktime"
	MimeJpeg  = "image/jpeg"
	MimeCsv   = "text/csv"

	TypeCommand = "command output"
	TypeImage   = "image"
	TypeVideo   = "video"
	TypeText    = "text"
	TypeData    = "data"
)

const (
	ptrRef   = "ptr.%d.href"
	ptrRole  = "ptr.%d.role"
	fileSize = "file.size"
	fileMD5  = "file.md5"

	FileDuration = "file.duration"
	FileRecord   = "file.numrec"
)

type MimeSet []Mime

func (ms MimeSet) Get(ext string) Mime {
	for _, m := range ms {
		if !m.isZero() && m.Accept(ext) {
			return m
		}
	}
	return Mime{}
}

type Mime struct {
	Extensions []string
	Mime       string
	Type       string
}

func (m Mime) Accept(ext string) bool {
	sort.Strings(m.Extensions)
	i := sort.SearchStrings(m.Extensions, ext)
	return i < len(m.Extensions) && m.Extensions[i] == ext
}

func (m Mime) isZero() bool {
	return m.Mime == "" && m.Type == "" && len(m.Extensions) == 0
}

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

func CreateLinkFrom(d Data) Link {
	file := filepath.Join(d.Resolve(), filepath.Base(d.File))
	return CreateLink(file, d.Type)
}

func CreateLink(n, r string) Link {
	return Link{
		File: n,
		Role: r,
	}
}

type Archive struct {
	DataDir string `toml:"datadir"`
	MetaDir string `toml:"metadir"`
}

func (a Archive) CreateFile(d Data, buf []byte) (Link, error) {
	var k Link
	d.File = filepath.Join(d.Resolve(), filepath.Base(d.File))
	if err := a.storeFile(d, buf); err != nil {
		return k, err
	}
	k.File = d.File
	k.Role = ""
	return k, a.storeMeta(d, d.File)
}

func (a Archive) Store(d Data) error {
	file := filepath.Join(d.Resolve(), filepath.Base(d.File))
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
	return EncodeData(w, d)
}

func (a Archive) storeFile(d Data, buf []byte) error {
	file := filepath.Join(a.DataDir, d.File)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	return ioutil.WriteFile(file, buf, 0644)
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

type Context struct {
	Experiment string
	Model      string
	Source     string
	Owner      string

	Increments []Increment `toml:"increment"`
	Metadata   []Parameter
}

func (c Context) Update(d Data) Data {
	if d.Experiment == "" {
		d.Experiment = c.Experiment
	}
	if d.Source == "" {
		d.Source = c.Source
	}
	if d.Model == "" {
		d.Model = c.Model
	}
	if d.Owner == "" {
		d.Owner = c.Owner
	}
	if !d.AcqTime.IsZero() && len(d.Increments) == 0 && len(c.Increments) > 0 {
		x := sort.Search(len(c.Increments), func(i int) bool {
			return c.Increments[i].Contains(d.AcqTime)
		})
		if x < len(c.Increments) && c.Increments[x].Contains(d.AcqTime) {
			d.Increments = append(d.Increments, c.Increments[x].Num)
		}
	}
	d.Parameters = append(d.Parameters, c.Metadata...)
	return d
}

type Data struct {
	Extensions []string
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
	Archive    Pattern

	Mimes MimeSet `toml:"mimetype"`

	Parameters []Parameter `toml:"metadata"`
	Links      []Link      `toml:"links"`

	Size int64
	MD5  string
}

func ReadFile(d *Data, file string) error {
	d.File = file
	f, err := os.Open(d.File)
	if err != nil {
		return err
	}
	defer f.Close()

	var r io.Reader = f
	if filepath.Ext(file) == ExtGZ {
		r, err = gzip.NewReader(r)
		if err != nil {
			return err
		}
	}

	m := d.Mimes.Get(filepath.Ext(file))
	if !m.isZero() {
		if d.Type == "" {
			d.Type = m.Type
		}
		if d.Mime == "" {
			d.Mime = m.Mime
		}
	}
	return ReadFrom(d, r)
}

func ReadFrom(d *Data, r io.Reader) error {
	var (
		sumSHA = sha256.New()
		sumMD5 = md5.New()
		err    error
	)
	if d.Size, err = io.Copy(io.MultiWriter(sumSHA, sumMD5), r); err != nil {
		return err
	}

	d.Integrity = SHA
	d.Sum = fmt.Sprintf("%x", sumSHA.Sum(nil))
	d.MD5 = fmt.Sprintf("%x", sumMD5.Sum(nil))

	return err
}

func (d Data) Resolve() string {
	if d.Archive.Resolver == nil {
		return ""
	}
	return d.Archive.Resolve(d)
}

func (d Data) Accept(file string) bool {
	if len(d.Extensions) == 0 {
		return false
	}
	sort.Strings(d.Extensions)
	var (
		e = filepath.Ext(file)
		i = sort.SearchStrings(d.Extensions, e)
	)
	return i < len(d.Extensions) && d.Extensions[i] == e
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
		h := MakeParameter(fmt.Sprintf(ptrRef, i+1), k.File)
		d.Parameters = append(d.Parameters, h)
		if k.Role != "" {
			r := MakeParameter(fmt.Sprintf(ptrRole, i+1), k.Role)
			d.Parameters = append(d.Parameters, r)
		}
	}
	if d.Size > 0 {
		d.Parameters = append(d.Parameters, MakeParameter(fileSize, d.Size))
	}
	if d.MD5 != "" {
		d.Parameters = append(d.Parameters, MakeParameter(fileMD5, d.MD5))
	}
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
