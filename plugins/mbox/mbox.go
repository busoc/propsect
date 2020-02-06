package main

import (
	"bufio"
	"hash"
	"io"
	"os"
	"strings"
	"time"

	"github.com/busoc/prospect"
	"github.com/midbel/mbox"
	"github.com/midbel/toml"
)

type filterFunc func(mbox.Message) bool

type predicate struct {
	From       string
	To         string
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
		Filter   []predicate
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
	var (
		i   prospect.FileInfo
		err error
	)

	return i, err
}

func (m *module) processMessage(msg mbox.Message) error {
	return nil
}

func (m *module) nextMessage() error {
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
			m.closer.Close()
			err = prospect.ErrDone
		}
		if err == nil && m.filter(msg) {
			break
		}
	}
	if err == nil {
		err = m.processMessage(msg)
	}
	return err
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
	str, accept := cmpStrings(from)
	return func(m mbox.Message) bool {
		return accept(m.From(), str)
	}
}

func withTo(to string) filterFunc {
	if to == "" {
		return keep
	}
	str, accept := cmpStrings(to)
	return func(m mbox.Message) bool {
		for _, to := range m.To() {
			if accept(to, str) {
				return true
			}
		}
		return false
	}
}

func withSubject(subj string) filterFunc {
	str, accept := cmpStrings(subj)
	return func(m mbox.Message) bool {
		return accept(m.Subject(), str)
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
	return func(m mbox.Message) bool {
		return !attach || m.HasAttachments()
	}
}

func keep(_ mbox.Message) bool {
	return true
}

func cmpStrings(str string) (string, func(string, string) bool) {
	if len(str) == 0 {
		return str, func(_, _ string) bool { return true }
	}
	var (
		not bool
		cmp func(string, string) bool
	)
	if str[0] == '!' {
		not, str = true, str[1:]
	}
	if str[0] == '~' {
		cmp, str = strings.Contains, str[1:]
	} else {
		cmp = func(str1, str2 string) bool { return str1 == str2 }
	}
	if not {
		return str, func(str1, str2 string) bool { return !cmp(str1, str2) }
	}
	return str, cmp
}
