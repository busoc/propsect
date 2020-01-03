package prospect

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

func Parse(str string) (resolver, error) {
	var (
		rs    []resolver
		parts = strings.Split(strings.Trim(str, "/"), "/")
	)
	for _, p := range parts {
		r, err := parse(p)
		if err != nil {
			return nil, err
		}
		rs = append(rs, r)
	}
	return path{rs: rs}, nil
}

const (
	lcurly = '{'
	rcurly = '}'
)

func parse(str string) (resolver, error) {
	var (
		offset int
		rs     []resolver
	)
	for offset < len(str) {
		start := strings.IndexByte(str[offset:], lcurly)
		if start < 0 {
			break
		}
		end := strings.IndexByte(str[offset+start:], rcurly)
		if end < 0 {
			return nil, fmt.Errorf("missing closing brace")
		}
		if end-start == 1 {
			return nil, fmt.Errorf("empty placeholder")
		}

		if q := str[offset : offset+start]; len(q) > 0 {
			rs = append(rs, literal(q))
		}
		offset += start + 1

		rs = append(rs, fragment{name: str[offset : offset+end-1]})
		offset += end
	}

	if len(str[offset:]) > 0 {
		rs = append(rs, literal(str[offset:]))
	}

	if len(rs) == 1 {
		return rs[0], nil
	}
	return compound{rs: rs}, nil
}

type resolver interface {
	Resolve(Data) string
	fmt.Stringer
}

type literal string

func (i literal) Resolve(_ Data) string {
	return string(i)
}

func (i literal) String() string {
	return fmt.Sprintf("literal(%s)", string(i))
}

type fragment struct {
	name string
}

func (f fragment) Resolve(dat Data) string {
	replace := func(str string) string {
		return strings.ReplaceAll(strings.Title(str), " ", "")
	}

	var str string
	switch strings.ToLower(f.name) {
	default:
		// str = "unknown"
	case levelSource:
		str = replace(dat.Source)
	case levelModel:
		str = replace(dat.Model)
	case levelMime, levelFormat:
		str = replace(parseMime(dat.Info.Mime))
	case levelType:
		str = replace(dat.Info.Type)
	case levelYear:
		str = strconv.Itoa(dat.Info.AcqTime.Year())
	case levelDoy:
		str = fmt.Sprintf("%03d", dat.Info.AcqTime.YearDay())
	case levelMonth:
		str = fmt.Sprintf("%02d", dat.Info.AcqTime.Month())
	case levelDay:
		str = fmt.Sprintf("%02d", dat.Info.AcqTime.Day())
	case levelHour:
		str = fmt.Sprintf("%02d", dat.Info.AcqTime.Hour())
	case levelMin:
		str = fmt.Sprintf("%02d", dat.Info.AcqTime.Minute())
	case levelSec:
		str = fmt.Sprintf("%02d", dat.Info.AcqTime.Second())
	case levelStamp:
		u := dat.Info.AcqTime.Unix()
		str = strconv.Itoa(int(u))
	}
	return str
}

func (f fragment) String() string {
	return fmt.Sprintf("fragment(%s)", f.name)
}

type compound struct {
	rs []resolver
}

func (c compound) Resolve(dat Data) string {
	var buf strings.Builder
	for _, r := range c.rs {
		buf.WriteString(r.Resolve(dat))
	}
	return buf.String()
}

func (c compound) String() string {
	var buf strings.Builder
	for _, r := range c.rs {
		buf.WriteString(r.String())
	}
	return fmt.Sprintf("compound(%s)", buf.String())
}

type path struct {
	rs []resolver
}

func (p path) Resolve(dat Data) string {
	str := make([]string, len(p.rs))
	for j := range p.rs {
		str[j] = p.rs[j].Resolve(dat)
	}
	return filepath.Join(str...)
}

func (p path) String() string {
	str := make([]string, len(p.rs))
	for j := range p.rs {
		str[j] = p.rs[j].String()
	}
	return fmt.Sprintf("path(%s)", filepath.Join(str...))
}
