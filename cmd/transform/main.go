package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	var (
		all  = flag.Bool("a", false, "print all")
		self = flag.Bool("k", false, "keep self including container")
		dir  = flag.String("d", "", "directory")
		clean = flag.Bool("c", false, "clean")
	)
	flag.Parse()

	var r io.Reader
	if flag.NArg() == 0 {
		r = os.Stdin
	} else {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
		r = f
	}
	data, err := readData(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	if *clean {
		cleanData(data)
	} else {
		updateData(data, *self)
	}
	if *dir == "" {
		printData(data, *all)
		return
	}
	if err := dumpData(data, *dir, *all); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type Entry struct {
	Name    string
	Include bool
}

type Container struct {
	Name     string
	Space    string
	Base     string
	Abstract bool
	Root     bool
	Fill     bool
	Entries  []Entry
}

func (c Container) Skip() bool {
	return c.Root || c.Base == "" || c.Abstract || len(c.Entries) == 0
}

func (c Container) Clone() Container {
	clone := c
	clone.Entries = make([]Entry, len(c.Entries))
	copy(clone.Entries, c.Entries)
	return clone
}

func readData(r io.Reader) ([]Container, error) {
	var data []Container
	return data, json.NewDecoder(r).Decode(&data)
}

func cleanData(data []Container) {
	for i, d := range data {
		es := make([]Entry, 0, len(d.Entries))
		for _, e := range d.Entries {
			if e.Include {
				continue
			}
			es = append(es, e)
		}
		data[i].Entries = data[i].Entries[:0]
		data[i].Entries = append(data[i].Entries, es...)
		data[i].Abstract = false
		data[i].Root = false
		data[i].Fill = true
	}
}

func updateData(data []Container, self bool) {
	sort.Slice(data, func(i, j int) bool {
		return data[i].Name < data[j].Name
	})
	for i, d := range data {
		if d.Abstract {
			continue
		}
		updateEntries(&data[i], data, self)
	}
}

func updateEntries(curr *Container, data []Container, self bool) {
	if curr.Fill {
		return
	}
	defer func() { curr.Fill = true }()

	var clone Container
	if curr.Base != "" {
		x := sort.Search(len(data), func(j int) bool {
			return data[j].Name >= curr.Base
		})
		if x >= len(data) || data[x].Name != curr.Base {
			return
		}

		clone = data[x].Clone()
		updateEntries(&clone, data, self)
		data[x] = clone
	}

	es := make([]Entry, len(clone.Entries)+len(curr.Entries))
	n := copy(es, clone.Entries)
	copy(es[n:], curr.Entries)

	curr.Entries = curr.Entries[:0]
	for _, e := range es {
		if e.Include && e.Name == curr.Name {
			if self {
				curr.Entries = append(curr.Entries, e)
			}
			continue
		}
		if e.Include {
			x := sort.Search(len(data), func(i int) bool { return data[i].Name >= e.Name })
			if x < len(data) && data[x].Name == e.Name {
				clone := data[x].Clone()
				updateEntries(&clone, data, self)
				data[x] = clone
				curr.Entries = append(curr.Entries, clone.Entries...)
			}
		} else {
			if strings.Contains(strings.ToLower(e.Name), "fsl_data") {
				continue
			}
			curr.Entries = append(curr.Entries, e)
		}
	}

	if !curr.Skip() {
		xs := make([]Container, len(data))
		copy(xs, data)
		sort.Slice(xs, func(i, j int) bool {
			return xs[i].Base < xs[j].Base
		})
		x := sort.Search(len(xs), func(j int) bool {
			return xs[j].Base >= curr.Name
		})
		curr.Abstract = x < len(xs) && xs[x].Base == curr.Name
		curr.Root = x < len(xs) && xs[x].Base == curr.Name
	}
}

func printData(data []Container, all bool) {
	for _, d := range data {
		if !all && d.Skip() {
			continue
		}
		fmt.Printf("%s/%s (%d)\n", d.Space, d.Name, len(d.Entries))
		printEntries(os.Stdout, d.Entries, true)
		fmt.Println()
	}
}

func dumpData(data []Container, dir string, all bool) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	dump := func(d Container) error {
		file := filepath.Join(dir, d.Space, d.Name+".txt")
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			return err
		}
		w, err := os.Create(file)
		if err != nil {
			return err
		}
		defer w.Close()
		return printEntries(w, d.Entries, false)
	}
	for _, d := range data {
		if !all && d.Skip() {
			continue
		}
		if err := dump(d); err != nil {
			return err
		}
	}
	return nil
}

func printEntries(w io.Writer, entries []Entry, keep bool) error {
	for _, e := range entries {
		if !keep && strings.HasPrefix(e.Name, "FSL_Data") {
			continue
		}
		prefix := "-"
		if e.Include {
			prefix = "*"
		}
		_, err := fmt.Fprintf(w, "%s %s\n", prefix, e.Name)
		if err != nil {
			return err
		}
	}
	return nil
}
