package main

import (
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"

	"github.com/busoc/prospect"
	"github.com/midbel/mbox"
	"github.com/midbel/toml"
)

const (
	mailSubject = "mail.subject"
	mailDesc    = "mail.description"
)

type module struct {
	cfg prospect.Config

	inner  *reader
	digest hash.Hash

	keep     bool
	handlers []handler
	queue    <-chan part
}

func New(cfg prospect.Config) (prospect.Module, error) {
	c := struct {
		Keep     bool      `toml:"keep-files"`
		Handlers []handler `toml:"mail"`
	}{}
	if err := toml.DecodeFile(cfg.Config, &c); err != nil {
		return nil, err
	}

	inner, err := readMessages(cfg.Location)
	if err != nil {
		return nil, err
	}

	m := module{
		inner:    inner,
		cfg:      cfg,
		digest:   cfg.Hash(),
		handlers: c.Handlers,
		keep:     c.Keep,
	}
	return &m, m.nextMessage()
}

func (m *module) String() string {
	return "mail"
}

func (m *module) Process() (prospect.FileInfo, error) {
	p, ok := <-m.queue
	if !ok {
		err := m.nextMessage()
		if err != nil {
			return prospect.FileInfo{}, err
		}
		return m.Process()
	}
	if p.Info.Type == "" {
		p.Info.Type = m.cfg.Type
	}
	if p.Info.Type == "" {
		p.Info.Type = prospect.TypeData
	}
	p.Info.Integrity = m.cfg.Integrity
	p.Info.Level = m.cfg.Level
	return p.Info, p.Err
}

func (m *module) nextMessage() error {
	var (
		msg  mbox.Message
		hdl  handler
		err  error
		done bool
	)
	for !done {
		msg, err = m.inner.nextMessage()
		if err == io.EOF {
			err = prospect.ErrDone
		}
		if err != nil {
			break
		}
		for _, hdl = range m.handlers {
			if done = hdl.Accept(msg); done {
				break
			}
		}
	}
	if err == nil {
		m.queue = m.processMessage(hdl, msg)
	}
	return err
}

func (m *module) processMessage(hdl handler, msg mbox.Message) <-chan part {
	queue := make(chan part)
	go func() {
		defer func() {
			close(queue)
			if !m.keep {
				os.RemoveAll(hdl.Maildir)
			}
		}()
		parts := hdl.items(msg)
		for _, pt := range parts {
			info := prospect.FileInfo{
				File:    pt.File,
				Type:    hdl.Type,
				Mime:    pt.Mime,
				AcqTime: msg.Date(),
				ModTime: msg.Date(),
			}
			info.Parameters = []prospect.Parameter{
				prospect.MakeParameter(mailSubject, msg.Subject()),
				prospect.MakeParameter(prospect.FileSize, fmt.Sprintf("%d", pt.Len())),
			}
			for _, p := range parts {
				if p.File == pt.File {
					continue
				}
				k := prospect.Link{
					File: filepath.Join(hdl.Maildir, p.File),
					Role: p.Role,
				}
				info.Links = append(info.Links, k)
			}
			if len(pt.Meta) > 0 {
				info.Parameters = append(info.Parameters, prospect.MakeParameter(mailDesc, pt.Meta))
			}
			err := os.MkdirAll(hdl.Maildir, 0755)
			if err == nil {
				err = m.writeFile(pt.File, pt.Part)
				info.Sum = fmt.Sprintf("%x", m.digest.Sum(nil))
			}
			queue <- part{
				Info: info,
				Err:  err,
			}
			m.digest.Reset()
		}
	}()
	return queue
}

func (m *module) writeFile(file string, p mbox.Part) error {

	w, err := os.Create(file)
	if err != nil {
		return err
	}
	defer w.Close()

	ws := io.MultiWriter(w, m.digest)
	_, err = ws.Write(p.Bytes())
	return err
}
