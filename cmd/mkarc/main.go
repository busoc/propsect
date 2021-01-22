package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/midbel/toml"
	"golang.org/x/sync/semaphore"
)

type writer struct {
	inner *os.File
	mu    sync.Mutex
}

func (w *writer) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.inner.Write(b)
}

func Writer(w io.Writer) io.Writer {
	return writer{inner: w}
}

var (
	Out = Writer(os.Stdout)
	Err = Writer(os.Stderr)
)

type Cmd struct {
	Path string
	File string
	Args []string
}

func (c Cmd) Exec() error {
	args := append(c.Args, c.File)
	cmd := exec.Command(c.Path, args...)
	cmd.Stdout = Out
	cmd.Stderr = Err

	return cmd.Run()
}

func main() {
	flag.Parse()

	c := struct {
		Task     int64
		Commands []Cmd
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
			defer sema.Release()
			c.Exec()
		}(c)
	}
	sema.Acquire(ctx, len(c.Commands))
}
