package main

import (
	"bufio"
	"fmt"
	"hash"
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
	Mime          string `toml:"content-type"`
	File          string `toml:"filename"`
	CaseSensitive bool   `toml:"case-sensitive"`
}

type predicate struct {
	From       string
	To         []string
	Subject    string
	NoReply    bool `toml:"no-reply"`
	Attachment bool

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
		withAttachment(p.Attachment),
	}
	return withFilter(fs...)
}

type module struct {
	cfg prospect.Config

	reader *bufio.Reader
	closer io.Closer
	digest hash.Hash

	datadir string
	keep    bool
	filter  filterFunc
}

func New(cfg prospect.Config) (prospect.Module, error) {
	c := struct {
		Maildir  string
		Keep     bool `toml:"keep-files"`
		File     string
		Metadata string
		Filter   []predicate `toml:"message"`
	}{}
	if err := toml.DecodeFile(cfg.Config, &c); err != nil {
		return nil, err
	}

	fs := make([]filterFunc, len(c.Filter))
	for i, f := range c.Filter {
		fs[i] = f.filter()
	}

	r, err := os.Open(cfg.Location)
	if err != nil {
		return nil, err
	}

	m := module{
		cfg:     cfg,
		reader:  bufio.NewReader(r),
		closer:  r,
		digest:  cfg.Hash(),
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
	var i prospect.FileInfo

	msg, err := m.nextMessage()
	if err != nil {
		return i, err
	}

	if i, err = m.processMessage(msg); err == nil {
		i.Integrity = m.cfg.Integrity
		i.Type = m.cfg.Type
		i.Level = m.cfg.Level
		// set i.Mime && i.Type
	}
	return i, err
}

func (m *module) processMessage(msg mbox.Message) (prospect.FileInfo, error) {
	var i prospect.FileInfo

	i.AcqTime = msg.Date()
	i.ModTime = msg.Date()

	fmt.Println(msg.From(), msg.Date(), msg.Subject())

	return i, prospect.ErrSkip
}

func (m *module) nextMessage() (mbox.Message, error) {
	var (
		msg mbox.Message
		err error
	)
	for err == nil {
		msg, err = mbox.ReadMessage(m.reader)
		if err == io.EOF {
			if !m.keep {
				os.RemoveAll(m.datadir)
			}
			err = prospect.ErrDone
		}
		if err == nil && m.filter(msg) {
			break
		}
	}
	return msg, err
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
	starts = starts.UTC()
	ends = ends.UTC()
	return func(m mbox.Message) bool {
		when := m.Date().UTC()
		if when.Before(starts) {
			return false
		}
		return !when.After(ends)
	}
}

func withAttachment(attach bool) filterFunc {
	const (
		filename   = "filename"
		attachment = "attachment"
	)
	return func(m mbox.Message) bool {
		return !attach || m.HasAttachments()
	}
}

func keep(_ mbox.Message) bool {
	return true
}
