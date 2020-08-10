package main

import (
	"bytes"
	"encoding/binary"
	"net"
	"net/url"
	"strings"
)

type Addr struct {
	Host [4]byte
	Port uint16
}

type State struct {
	Local  Addr
	Remote Addr
	Size   int64
	Curr   int64
	Written int64
	File   [256]byte
}

func (s *State) Filename() string {
	x := bytes.Trim(s.File[:], "\x00")
	return string(x)
}

type Reporter struct {
	conn  net.Conn
	state State
}

func Report(addr string, local, remote net.Addr) (*Reporter, error) {
	c, err := net.Dial(splitAddr(addr))
	if err != nil {
		return nil, err
	}
	r := Reporter{conn: c}
	if a, ok := local.(*net.TCPAddr); ok {
		r.state.Local = Addr{
			Port: uint16(a.Port),
		}
		copy(r.state.Local.Host[:], a.IP.To4())
	}
	if a, ok := remote.(*net.TCPAddr); ok {
		r.state.Remote = Addr{
			Port: uint16(a.Port),
		}
		copy(r.state.Remote.Host[:], a.IP.To4())
	}
	return &r, nil
}

func (r *Reporter) Write(b []byte) (int, error) {
	n := len(b)
	if n > 0 {
		r.state.Written = int64(n)
		r.state.Curr += r.state.Written
		binary.Write(r.conn, binary.BigEndian, r.state)
	}
	return n, nil
}

func (r *Reporter) Start(file string, size int64) error {
	r.state.Curr = 0
	r.state.Size = size
	n := copy(r.state.File[:], file)
	for n < len(r.state.File) {
		r.state.File[n] = 0
		n++
	}

	return binary.Write(r.conn, binary.BigEndian, r.state)
}

func (r *Reporter) Close() error {
	return r.conn.Close()
}

func splitAddr(str string) (string, string) {
	u, err := url.Parse(str)
	if err != nil {
		return "udp", str
	}
	switch scheme := strings.ToLower(u.Scheme); scheme {
	case "tcp", "udp":
		return scheme, u.Host
	default:
		return "udp", u.Host
	}
}
