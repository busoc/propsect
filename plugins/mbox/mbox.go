package main

import (
	"bufio"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/busoc/prospect"
	"github.com/midbel/mbox"
	"github.com/midbel/toml"
)

const (
	mailSubject = "mail.subject"
	mailDesc    = "mail.description"
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

type include struct {
	Types   []string `toml:"content-type"`
	Pattern string
}

type part struct {
	Info prospect.FileInfo
	Err  error
}

type handler struct {
	Type     string
	Maildir  string
	Metadata string

	Predicate predicate `toml:"predicate"`
	Includes  []include `toml:"file"`

	filter filterFunc
}

func (h *handler) Accept(msg mbox.Message) bool {
	if h.filter == nil {
		h.filter = buildFilter(h.Predicate)
	}
	return h.filter(msg)
}

type module struct {
	cfg prospect.Config

	reader *bufio.Reader
	closer io.Closer
	digest hash.Hash

	handlers []handler
	queue    <-chan part
}

func New(cfg prospect.Config) (prospect.Module, error) {
	c := struct {
		Keep     bool      `toml:"keep-files"`
		Handlers []handler `toml:"mail"`
	}{}
	if err := toml.DecodeFile(cfg.Config, &c); err != nil {
		return nil, err
	}

	r, err := os.Open(cfg.Location)
	if err != nil {
		return nil, err
	}

	m := module{
		reader:   bufio.NewReader(r),
		closer:   r,
		cfg:      cfg,
		digest:   cfg.Hash(),
		handlers: c.Handlers,
	}
	return &m, m.nextMessage()
}

func (m *module) String() string {
	return "mail"
}

func (m *module) Process() (prospect.FileInfo, error) {
	p, ok := <-m.queue
	if !ok {
		err := m.nextMessage()
		if err != nil {
			return prospect.FileInfo{}, err
		}
		return m.Process()
	}
	if p.Info.Type == "" {
		p.Info.Type = m.cfg.Type
	}
	return p.Info, p.Err
}

func (m *module) nextMessage() error {
	var (
		msg  mbox.Message
		hdl  handler
		err  error
		done bool
	)
	for !done {
		msg, err = mbox.ReadMessage(m.reader)
		if err == io.EOF {
			m.closer.Close()
			err = prospect.ErrDone
		}
		if err != nil {
			break
		}
		for _, hdl = range m.handlers {
			if done = hdl.Accept(msg); done {
				break
			}
		}
	}
	if err == nil {
		m.queue = m.processMessage(hdl, msg)
	}
	return err
}

func (m *module) processMessage(hdl handler, msg mbox.Message) <-chan part {
	queue := make(chan part)
	go func() {
		defer close(queue)
		var (
			p    = msg.Part(hdl.Metadata)
			meta = p.Text()
		)
		for _, i := range hdl.Includes {
			var (
				mt string
				pt mbox.Part
			)
			for _, a := range i.Types {
				pt, mt = msg.Part(a), a
				if pt.Len() > 0 {
					break
				}
			}
			if pt.Len() == 0 {
				continue
			}
			match, _ := regexp.MatchString(i.Pattern, pt.Filename())
			if i.Pattern != "" && !match {
				continue
			}
			file := filepath.Join(hdl.Maildir, pt.Filename())
			info := prospect.FileInfo{
				File:    file,
				Type:    hdl.Type,
				Mime:    mt,
				AcqTime: msg.Date(),
				ModTime: msg.Date(),
			}
			info.Parameters = append(info.Parameters, prospect.MakeParameter(mailSubject, msg.Subject()))
			if len(meta) > 0 {
				info.Parameters = append(info.Parameters, prospect.MakeParameter(mailDesc, string(meta)))
			}
			err := os.MkdirAll(hdl.Maildir, 0755)
			if err == nil {
				err = ioutil.WriteFile(file, pt.Bytes(), 0644)
			}
			queue <- part{
				Info: info,
				Err:  err,
			}
		}
	}()
	return queue
}

func buildFilter(p predicate) filterFunc {
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
