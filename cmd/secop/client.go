package main

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/juju/ratelimit"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const DefaultBufferSize = 1024

type Directory struct {
	Local     string
	Remote    string
	Keep      bool
	Force     bool
	Compress  bool
	Integrity bool
}

type Credential struct {
	Addr   string
	User   string
	Passwd string

	Limit   int64
	Monitor string
}

func (c Credential) Connect() (*Client, error) {
	cfg := ssh.ClientConfig{
		User: c.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(c.Passwd),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := ssh.Dial("tcp", c.Addr, &cfg)
	if err != nil {
		return nil, err
	}
	client, err := sftp.NewClient(conn)
	if err != nil {
		return nil, err
	}
	remote := Client{
		limit:  c.Limit,
		client: client,
	}
	if c.Monitor != "" {
		r, err := Report(c.Monitor, conn.LocalAddr(), conn.RemoteAddr())
		if err != nil {
			return nil, err
		}
		remote.conn = r
	}
	return &remote, nil
}

type Client struct {
	limit  int64
	client *sftp.Client

	conn *Reporter
}

func (c *Client) Copy(d Directory, buffer int64) error {
	if buffer <= 0 {
		buffer = DefaultBufferSize
	}
	var (
		local  = filepath.Clean(strings.TrimSpace(d.Local))
		remote = filepath.Clean(strings.TrimSpace(d.Remote))
		buf    = make([]byte, buffer)
	)
	if remote == "=" {
		remote = local
	}
	err := filepath.Walk(d.Local, func(file string, i os.FileInfo, err error) error {
		if err != nil || i.IsDir() {
			return err
		}
		r, err := os.Open(file)
		if err != nil {
			return err
		}
		defer func(file string) {
			r.Close()
			if !d.Keep {
				os.Remove(file)
			}
		}(file)

		rfile := filepath.Join(remote, strings.TrimPrefix(file, local))
		log.Printf("begin transfer file: %s -> %s", file, rfile)
		if !d.Force {
			if j, err := c.client.Stat(rfile); err == nil && !i.IsDir() {
				var (
					mtime = i.ModTime().Truncate(time.Second)
					rtime = j.ModTime().Truncate(time.Second)
				)
				if i.Size() == j.Size() && mtime.Equal(rtime) {
					log.Printf("skip transfer file: %s", file)
					return nil
				}
			}
		}
		n, err := c.copy(r, i, rfile, d.Compress, buf)
		if err != nil {
			log.Printf("error transfer file: %s -> %s: %s", file, rfile, err)
		}
		log.Printf("end transfer file: %s -> %s (%d bytes)", file, rfile, n)
		return nil
	})
	return err
}

func (c *Client) copy(r io.Reader, i os.FileInfo, file string, minify bool, buffer []byte) (int64, error) {
	if minify {
		file += ".gz"
	}
	if err := c.client.MkdirAll(filepath.Dir(file)); err != nil {
		return 0, err
	}
	var w io.Writer
	if f, err := c.client.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC); err != nil {
		return 0, err
	} else {
		w = f
		defer func() {
			f.Close()
			c.client.Chtimes(file, i.ModTime(), i.ModTime())
		}()
	}
	if minify {
		z, _ := gzip.NewWriterLevel(w, gzip.BestCompression)
		defer z.Close()

		w = z
	}

	var rs io.Reader = r
	if c.conn != nil {
		c.conn.Start(file, i.Size())
		rs = io.TeeReader(rs, c.conn)
	}

	if c.limit > 0 {
		limit := float64(c.limit)
		w = ratelimit.Writer(w, ratelimit.NewBucketWithRate(limit, c.limit))
	}

	var (
		local  = md5.New()
		remote = md5.New()
	)
	n, err := io.CopyBuffer(io.MultiWriter(w, remote), io.TeeReader(rs, local), buffer)
	if err != nil {
		return n, err
	}
	if z := i.Size(); n != z {
		return n, fmt.Errorf("%w: %d - %d", ErrSize, z, n)
	}
	if c1, c2 := local.Sum(nil), remote.Sum(nil); !bytes.Equal(c1[:], c2[:]) {
		return n, fmt.Errorf("%w: %x - %x", ErrMismatched, c1, c2)
	}
	return n, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		c.conn.Close()
	}
	return c.client.Close()
}
