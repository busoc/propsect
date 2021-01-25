package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/midbel/toml"
	"golang.org/x/sync/semaphore"
)

type writer struct {
	inner io.Writer
	mu    sync.Mutex
}

func (w *writer) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.inner.Write(b)
}

func Writer(w io.Writer) io.Writer {
	return &writer{inner: w}
}

var (
	Out = Writer(os.Stdout)
	Err = Writer(os.Stderr)
)

type Cmd struct {
	Path string
	File string
	Args []string

	Pre  []Cmd `toml:"pre"`
	Post []Cmd `toml:"post"`
}

func (c Cmd) Exec() error {
	if err := execCmd(c.Pre...); err != nil {
		return err
	}
	if err := execCmd(c); err != nil {
		return err
	}
	return execCmd(c.Post...)
}

func (c Cmd) Run() error {
	args := append(c.Args, c.File)
	cmd := exec.Command(c.Path, args...)
	cmd.Stdout = Out
	cmd.Stderr = Err

	return cmd.Run()
}

func execCmd(cs ...Cmd) error {
	var err error
	for _, c := range cs {
		err = c.Run()
		if err != nil {
			break
		}
	}
	return err
}

func main() {
	flag.Parse()

	c := struct {
		Task     int64 `toml:"parallel"`
		Commands []Cmd `toml:"command"`
	}{}
	if err := toml.DecodeFile(flag.Arg(0), &c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if c.Task == 0 {
		c.Task = int64(len(c.Commands))
	}
	var (
		ctx  = context.TODO()
		sema = semaphore.NewWeighted(c.Task)
	)
	for _, c := range c.Commands {
		if err := sema.Acquire(ctx, 1); err != nil {
			os.Exit(2)
		}
		go func(c Cmd) {
			defer func(n time.Time) {
				sema.Release(1)
				log.Printf("done: %s %s (%s)", c.Path, c.File, time.Since(n))
			}(time.Now())
			log.Printf("start: %s %s", c.Path, c.File)
			if err := c.Exec(); err != nil {
				log.Printf("error: %s", err)
			}
		}(c)
	}
	sema.Acquire(ctx, c.Task)
}
