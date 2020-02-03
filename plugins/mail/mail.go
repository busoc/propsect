package main

import (
	"bufio"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/busoc/prospect"
	"github.com/midbel/mbox"
	"github.com/midbel/toml"
)

type filterFunc func(mbox.Message) bool

type attachment struct {
	Mime string `toml:"mimetype"`
	File string `toml:"filename"`
}

type predicate struct {
	From    string
	To      []string
	Subject string
	NoReply bool `toml:"no-reply"`

	Attachments []attachment

	Starts time.Time `toml:"dtstart"`
	Ends   time.Time `toml:"dtend"`
}

func (p predicate) filter() filterFunc {
	fs := []filterFunc{
		withFrom(p.From),
		withTo(p.To),
		withSubject(p.Subject),
		withReply(p.NoReply),
		withInterval(p.Starts, p.Ends),
		withAttachment(p.Attachments),
	}
	return withFilter(fs...)
}

/*
maildir    = "/var/mail"
keep-files = true

[[filter]]
from        = "fsl_ops@busoc.be"
subject     = "Daily Operations Report for FSL"
dtstart     = 2018-07-22
dtend       = 2019-06-19
attachments = [
	{mimetype = "application/pdf", filename = "compgran"},
	{mimetype = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", filename = "compgran"},
]
*/

type module struct {
	cfg prospect.Config

	reader *bufio.Reader
	closer io.Closer

	datadir string
	keep    bool
	filter  filterFunc
}

func New(cfg prospect.Config) (prospect.Module, error) {
	c := struct {
		Maildir string
		Keep    bool `toml:"keep-files"`
		Filter  []predicate
	}{}
	if err := toml.DecodeFile(cfg.Config, &c); err != nil {
		return nil, err
	}

	fs := make([]filterFunc, len(c.Filter))
	for i, f := range c.Filter {
		fs[i] = f.filter()
	}

	r, err := os.Open(cfg.Location)
	if err == nil {
		return nil, err
	}

	m := module{
		cfg:     cfg,
		reader:  bufio.NewReader(r),
		closer:  r,
		filter:  withFilter(fs...),
		datadir: c.Maildir,
		keep:    c.Keep,
	}
	return &m, nil
}

func (m *module) String() string {
	return "mail"
}

func (m *module) Process() (prospect.FileInfo, error) {
	var (
		msg mbox.Message
		err error
	)
	for {
		msg, err = mbox.ReadMessage(m.reader)
		if err != nil {
			if err == io.EOF {
				if !m.keep {
					os.RemoveAll(m.datadir)
				}
				return prospect.FileInfo{}, prospect.ErrDone
			}
		}
		if m.filter(msg) {
			break
		}
	}
	return m.processMessage(msg)
}

func (m *module) processMessage(msg mbox.Message) (prospect.FileInfo, error) {
	return prospect.FileInfo{}, prospect.ErrSkip
}

func withFilter(funcs ...filterFunc) filterFunc {
	return func(m mbox.Message) bool {
		for _, fn := range funcs {
			if !fn(m) {
				return false
			}
		}
		return true
	}
}

func withFrom(from string) filterFunc {
	return func(m mbox.Message) bool {
		return from == "" || m.From() == from
	}
}

func withTo(to []string) filterFunc {
	if len(to) == 0 {
		return keep
	}
	sort.Strings(to)
	return func(m mbox.Message) bool {
		for _, t := range m.To() {
			i := sort.SearchStrings(to, t)
			if i < len(to) && to[i] == t {
				return true
			}
		}
		return false
	}
}

func withSubject(subj string) filterFunc {
	return func(m mbox.Message) bool {
		return subj == "" || strings.Contains(m.Subject(), subj)
	}
}

func withReply(noreply bool) filterFunc {
	return func(m mbox.Message) bool {
		if noreply {
			return !m.IsReply()
		}
		return true
	}
}

func withInterval(starts, ends time.Time) filterFunc {
	if starts.IsZero() && ends.IsZero() {
		return keep
	}
	return func(m mbox.Message) bool {
		when := m.Date()
		if when.Before(starts) {
			return false
		}
		return !when.After(ends)
	}
}

func withAttachment(as []attachment) filterFunc {
	if len(as) == 0 {
		return keep
	}
	const (
		filename   = "filename"
		attachment = "attachment"
	)
	return func(m mbox.Message) bool {
		ps := m.Filter(func(hdr mbox.Header) bool {
			var (
				ct     = hdr.Get("content-type")
				df, ps = hdr.Split("content-disposition")
			)
			if df != attachment || len(ps) == 0 {
				return false
			}
			for _, a := range as {
				if a.Mime != "" && !strings.HasPrefix(ct, a.Mime) {
					continue
				}
				if a.File != "" && strings.Contains(ps[filename], a.File) {
					return true
				}
			}
			return false
		})
		return len(ps) > 0
	}
}

func keep(_ mbox.Message) bool {
	return true
}
