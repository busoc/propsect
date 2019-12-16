package prospect

import (
	"sort"
)

const (
	MimeOctetDefault  = "application/octet-stream"
	MimeOctetUnformat = "application/octet-stream;access=sequential,form=unformatted"

	MimePlainDefault = "text/plain"
	MimeICN          = "text/plain;access=sequential;form=block-formatted;type=icn"
)

const (
	TypeData         = "data"
	TypeUplinkFile   = "uplink file"
	TypeUplinkNote   = "intre console note"
	TypeHighRateData = "high rate data"
	TypeRawTelemetry = "medium rate telemetry"
)

type Mime struct {
	Extensions []string
	Mime       string `toml:"mime"`
	Type       string
}

func (m *Mime) Has(ext string) (string, string, bool) {
	if !sort.StringsAreSorted(m.Extensions) {
		sort.Strings(m.Extensions)
	}
	var (
		x    = sort.SearchStrings(m.Extensions, ext)
		ok   = x < len(m.Extensions) && m.Extensions[x] == ext
		mime string
		data string
	)
	if ok {
		mime, data = m.Mime, m.Type
	}
	return mime, data, ok
}
