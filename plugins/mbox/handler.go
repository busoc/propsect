package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"github.com/busoc/prospect"
	"github.com/midbel/mbox"
)

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
	Role    string
}

type part struct {
	Info prospect.FileInfo
	Err  error
}

type item struct {
	Mime string
	File string
	Meta string
	Role string
	mbox.Part
}

type handler struct {
	Prefix   string
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

func (h *handler) items(msg mbox.Message) []item {
	var (
		p     = msg.Part(h.Metadata)
		meta  = p.Text()
		parts []item
	)
	for _, i := range h.Includes {
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
		file := pt.Filename()
		if file == "" {
			file = fmt.Sprintf("%s%s.eml", h.Prefix, msg.Date().Format("20060102_150405"))
		}
		file = filepath.Join(h.Maildir, file)

		j := item{
			Mime: mt,
			File: file,
			Meta: string(meta),
			Part: pt,
			Role: i.Role,
		}
		parts = append(parts, j)
	}
	return parts
}
