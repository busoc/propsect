package main

import (
	"bytes"
	"crypto/md5"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/midbel/toml"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var ErrMismatched = errors.New("checksum mismatched")

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
		local  = filepath.Clean(d.Local)
		remote = filepath.Clean(d.Remote)
	)
	return filepath.Walk(d.Local, func(file string, i os.FileInfo, err error) error {
		if err != nil || i.IsDir() {
			return err
		}
		r, err := os.Open(file)
		if err != nil {
			return err
		}
		defer r.Close()

		file = filepath.Join(remote, strings.TrimPrefix(local, file))
		if j, err := c.client.Stat(file); err == nil && !i.IsDir() {
			if i.Size() == j.Size() && i.ModTime().Equal(j.ModTime()) {
				return nil
			}
		}
		return c.copy(r, i, file)
	})
}

func (c *Client) copy(r io.Reader, i os.FileInfo, file string) error {
	if err := c.client.MkdirAll(filepath.Dir(file)); err != nil {
		return err
	}
	w, err := c.client.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer func() {
		w.Close()
		c.client.Chtimes(file, i.ModTime(), i.ModTime())
	}()
	var (
		local  = md5.New()
		remote = md5.New()
	)
	if _, err := io.Copy(io.MultiWriter(w, remote), io.TeeReader(r, local)); err != nil {
		return err
	}
	if c1, c2 := local.Sum(nil), remote.Sum(nil); !bytes.Equal(c1[:], c2[:]) {
		return fmt.Errorf("%w: %x - %x", ErrMismatched, c1, c2)
	}
	return nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

type Directory struct {
	Local  string
	Remote string
}

func main() {
	flag.Parse()

	c := struct {
		Credential
		Directories []Directory `toml:"directory"`
	}{}
	if err := toml.DecodeFile(flag.Arg(0), &c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	client, err := c.Connect()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer client.Close()

	for _, d := range c.Directories {
		if err := client.Copy(d); err != nil {
			fmt.Fprintf(os.Stderr, "fail to copy from %s to %s: %v", d.Local, d.Remote, err)
		}
	}
}
