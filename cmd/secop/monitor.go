package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
  "path/filepath"

	"github.com/midbel/cli"
	"github.com/midbel/wip"
	"golang.org/x/net/ipv4"
)

func runMonitor(cmd *cli.Command, args []string) error {
	nic := cmd.Flag.String("i", "", "interface")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	rc, err := NewReader(cmd.Flag.Arg(0), *nic)
	if err != nil {
		return err
	}
	defer rc.Close()

	var (
		curr State
		file string
		bar  *wip.Bar
	)
	for {
		if err := binary.Read(rc, binary.BigEndian, &curr); err != nil {
			return err
		}
		if file == "" || file != curr.Filename() {
      fmt.Println()
			bar = wip.Default(filepath.Base(curr.Filename()), curr.Size)
		}
		file = curr.Filename()
		bar.Update(curr.Curr)
	}
	return nil
}

type Reader struct {
	conn  net.PacketConn
	pc    *ipv4.PacketConn
	group *net.UDPAddr
}

func NewReader(addr, nic string) (io.ReadCloser, error) {
	var (
		r    Reader
		port string
		err  error
	)

	if _, port, err = net.SplitHostPort(addr); err != nil {
		return nil, err
	}

	if r.group, err = net.ResolveUDPAddr("udp", addr); err != nil {
		return nil, err
	}
	if r.conn, err = net.ListenPacket("udp4", "0.0.0.0:"+port); err != nil {
		return nil, err
	}
	r.pc = ipv4.NewPacketConn(r.conn)

	var ifi *net.Interface
	if nic != "" {
		ifi, err = net.InterfaceByName(nic)
		if err != nil {
			return nil, err
		}
	}
	if err = r.pc.JoinGroup(ifi, r.group); err != nil {
		return nil, err
	}
	r.pc.SetControlMessage(ipv4.FlagDst, true)
	return &r, nil
}

func (r *Reader) Read(b []byte) (int, error) {
	for {
		n, cm, _, err := r.pc.ReadFrom(b)
		if err != nil {
			return 0, err
		}
		if cm != nil {
			if cm.Dst.IsMulticast() && cm.Dst.Equal(r.group.IP) {
				return n, err
			}
		} else {
			return n, err
		}
	}
}

func (r *Reader) Close() error {
	err := r.pc.Close()
	if e := r.conn.Close(); e != nil {
		err = e
	}
	return err
}
