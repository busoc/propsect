package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/midbel/toml"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

var (
	ErrMismatched = errors.New("checksum mismatched")
	ErrSize       = errors.New("size mismatched")
)

type Credential struct {
	Addr   string
	User   string
	Passwd string
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
	return &Client{client}, nil
}

type Client struct {
	client *sftp.Client
}

func (c *Client) Copy(d Directory) error {
	var (
		local  = filepath.Clean(strings.TrimSpace(d.Local))
		remote = filepath.Clean(strings.TrimSpace(d.Remote))
	)
	if remote == "=" {
		remote = local
	}
	return filepath.Walk(d.Local, func(file string, i os.FileInfo, err error) error {
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
		n, err := c.copy(r, i, rfile, d.Compress)
		if err != nil {
			log.Printf("error transfer file: %s -> %s: %s", file, rfile, err)
		}
		log.Printf("end transfer file: %s -> %s (%d bytes)", file, rfile, n)
		return nil
	})
}

func (c *Client) copy(r io.Reader, i os.FileInfo, file string, minify bool) (int64, error) {
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
	var (
		local  = md5.New()
		remote = md5.New()
	)
	n, err := io.Copy(io.MultiWriter(w, remote), io.TeeReader(r, local))
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
	return c.client.Close()
}

type Directory struct {
	Local  string
	Remote string
	Keep   bool
	Force  bool
	Compress bool
}

func main() {
	quiet := flag.Bool("q", false, "quiet")
	flag.Parse()

	if *quiet {
		log.SetOutput(ioutil.Discard)
	}

	c := struct {
		Jobs int64
		Credential
		Directories []Directory `toml:"directory"`
	}{}
	if err := toml.DecodeFile(flag.Arg(0), &c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var err error
	if c.Jobs <= 1 {
		err = singleJob(c.Credential, c.Directories)
	} else {
		err = multiJobs(c.Credential, c.Directories, c.Jobs)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func multiJobs(c Credential, dirs []Directory, jobs int64) error {
	var (
		ctx  = context.TODO()
		sema = semaphore.NewWeighted(jobs)
		grp  errgroup.Group
	)
	defer sema.Acquire(ctx, jobs)
	for i := range dirs {
		if err := sema.Acquire(ctx, 1); err != nil {
			return err
		}
		d := dirs[i]
		grp.Go(func() error {
			defer sema.Release(1)
			client, err := c.Connect()
			if err != nil {
				return err
			}
			defer client.Close()
			return client.Copy(d)
		})
	}
	return grp.Wait()
}

func singleJob(c Credential, dirs []Directory) error {
	client, err := c.Connect()
	if err != nil {
		return err
	}
	defer client.Close()

	for _, d := range dirs {
		if err := client.Copy(d); err != nil {
			return fmt.Errorf("fail to copy from %s to %s: %v", d.Local, d.Remote, err)
		}
	}
	return nil
}
