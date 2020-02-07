package main

import (
	"regexp"
	"strings"
	"time"

	"github.com/midbel/mbox"
)

type filterFunc func(mbox.Message) bool

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
	re, err := regexp.Compile(subj)
	if err != nil {
		return keep
	}
	return func(m mbox.Message) bool {
		return re.MatchString(m.Subject())
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
