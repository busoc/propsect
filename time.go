package prospect

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func FormatDurationISO(d time.Duration) string {
	var (
		sdt  = uint64(d.Seconds())
		sec  uint64
		min  uint64
		hour uint64
		day  uint64
	)

	sec = sdt % 60
	sdt /= 60
	min = sdt % 60
	sdt /= 60
	hour = sdt % 24
	day = sdt / 24

	str := []byte("P")
	if day > 0 {
		str = strconv.AppendUint(str, day, 10)
		str = append(str, 'D')
	}
	if hour > 0 || min > 0 || sec > 0 {
		str = append(str, 'T')
	}
	if hour > 0 {
		str = strconv.AppendUint(str, hour, 10)
		str = append(str, 'H')
	}
	if min > 0 {
		str = strconv.AppendUint(str, min, 10)
		str = append(str, 'M')
	}
	if sec > 0 {
		str = strconv.AppendUint(str, sec, 10)
		str = append(str, 'S')
	}
	return string(str)
}

const (
	TimeFormatRT       = "rt"
	TimeFormatHDKLong  = "hadock"
	TimeFormatHDKShort = "hdk"
	TimeFormatNow      = "now"
	TimeFormatYD       = "year.doy"
	TimeFormatYDH      = "year.doy.hour"
)

type TimeFunc struct {
	parseTime func(string) (time.Time, error)
}

func (tp *TimeFunc) Set(str string) error {
	switch strings.ToLower(str) {
	case "", TimeFormatNow:
		tp.parseTime = TimeNow
	case TimeFormatRT, TimeFormatYDH:
		tp.parseTime = TimeRT
	case TimeFormatHDKLong, TimeFormatHDKShort:
		tp.parseTime = TimeHDK
	case TimeFormatYD:
		tp.parseTime = TimeYearDoy
	default:
		return fmt.Errorf("%s: unknown format", str)
	}
	return nil
}

func (tp *TimeFunc) GetTime(file string) (time.Time, error) {
	var (
		when time.Time
		err  error
	)
	if tp.parseTime != nil {
		when, err = tp.parseTime(file)
	}
	return when, err
}

const (
	patHdk = "20060102150405"
	patRt  = "2006-002-15-04"
	patYd  = "2006-002"
)

func TimeYearDoy(file string) (time.Time, error) {
	str := timeFromFile(file, level1, true)
	return time.Parse(patYd, str)
}

func TimeNow(_ string) (time.Time, error) {
	return time.Now(), nil
}

func TimeRT(file string) (time.Time, error) {
	var (
		str   = timeFromFile(file, level3, false)
		parts = strings.Split(filepath.Base(file), "_")
	)
	return time.Parse(patRt, fmt.Sprintf("%s-%s", str, parts[1]))
}

func TimeHDK(file string) (time.Time, error) {
	parts := strings.Split(filepath.Base(file), "_")
	return time.Parse(patHdk, parts[len(parts)-3]+parts[len(parts)-2])
}

const (
	level1 = iota + 1
	level2
	level3
)

func timeFromFile(file string, levels int, all bool) string {
	dir, base := filepath.Split(file)
	var parts []string
	if all {
		parts = append(parts, cleanExtension(base))
	}
	dir = filepath.Clean(dir)
	for i := 0; i < levels && dir != ""; i++ {
		dir, file = filepath.Split(dir)
		parts = append(parts, file)
		dir = filepath.Clean(dir)
	}
	return reverseJoin(parts)
}

func reverseJoin(parts []string) string {
	sort.Slice(parts, func(i, j int) bool { return i > j })
	return strings.Join(parts, "-")
}

func cleanExtension(file string) string {
	for {
		e := filepath.Ext(file)
		if e == "" {
			break
		}
		file = strings.TrimSuffix(file, e)
	}
	return file
}
