package prospect

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	HadockTime = "hadock"
	RTTime     = "rt"
)

type TimeFunc func(string) time.Time

func TimeRT(file string) time.Time {
	var (
		parts = make([]int, 3)
		dir   = filepath.Dir(file)
		base  = filepath.Base(file)
		when  time.Time
	)
	for i := len(parts) - 1; i >= 0; i-- {
		d, f := filepath.Split(dir)
		x, err := strconv.Atoi(f)
		if err != nil {
			return when
		}
		parts[i] = x
		dir = filepath.Dir(d)
	}
	when = when.AddDate(parts[0]-1, 0, parts[1]).Add(time.Duration(parts[2]) * time.Hour)
	if x := strings.Index(base, "_"); x >= 0 {
		base = base[x+1:]
		if x = strings.Index(base, "_"); x >= 0 {
			x, err := strconv.Atoi(base[:x])
			if err == nil {
				when = when.Add(time.Duration(x) * time.Minute)
			}
		}
	}
	return when.UTC()
}

func TimeHadock(file string) time.Time {
	ps := strings.Split(filepath.Base(file), "_")

	when, _ := time.Parse("20060102150405", ps[len(ps)-3]+ps[len(ps)-2])
	return when
}
