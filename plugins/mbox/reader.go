package main

import (
	"bufio"
	"io"
	"os"

	"github.com/midbel/glob"
	"github.com/midbel/mbox"
)

type reader struct {
	source *glob.Glob

	inner  *bufio.Reader
	closer io.Closer
}

func readMessages(location string) (*reader, error) {
	src, err := glob.New(location)
	if err != nil {
		return nil, err
	}
	r := reader{
		source: src,
	}
	return &r, r.reset()
}

func (r *reader) nextMessage() (mbox.Message, error) {
	msg, err := mbox.ReadMessage(r.inner)
	if err != nil {
		if err == io.EOF {
			err = r.reset()
		}
		return msg, err
	}
	return msg, err
}

func (r *reader) reset() error {
	if r.closer != nil {
		r.closer.Close()
	}
	file := r.source.Glob()
	if file == "" {
		return io.EOF
	}
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	if r.inner == nil {
		r.inner = bufio.NewReader(f)
	} else {
		r.inner.Reset(f)
	}
	return nil
}
