package prospect

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	CmdName = "command.name"
	CmdArgs = "command.args"
	CmdVersion = "command.version"
)

type Command struct {
	Path       string
	Version    string
	Args       []string
	Mime       string
	Type       string
	Ext        string
	Extensions []string
}

func (c Command) Exec(d Data) (Data, []byte, error) {
	if !c.accept(filepath.Ext(d.File)) {
		return d, nil, nil
	}
	var (
		args   = append(c.Args, d.File)
		cmd    = exec.Command(c.Path, args...)
		sumSHA = sha256.New()
		sumMD5 = md5.New()
		buf    bytes.Buffer
	)
	cmd.Stdout = io.MultiWriter(&buf, sumSHA, sumMD5)
	err := cmd.Run()
	if err != nil {
		return d, nil, err
	}

	d.Integrity = SHA
	d.Sum = fmt.Sprintf("%x", sumSHA.Sum(nil))
	d.MD5 = fmt.Sprintf("%x", sumMD5.Sum(nil))
	d.Size = int64(buf.Len())
	d.Level = 1
	d.Type = c.Type
	d.Mime = c.Mime
	d.File = d.File + c.Ext
	d.ModTime = time.Now()

	ps := []Parameter{
		MakeParameter(CmdName, filepath.Base(c.Path)),
		MakeParameter(CmdArgs, strings.Join(args, " ")),
	}
	d.Parameters = append(d.Parameters, ps...)

	if c.Version != "" {
		cmd = exec.Command(c.Path, c.Version)
		if buf, err := cmd.Output(); err == nil && len(buf) > 0 {
			d.Parameters = append(d.Parameters, MakeParameter(CmdVersion, string(buf)))
		}
	}
	return d, buf.Bytes(), nil
}

func (c Command) accept(ext string) bool {
	sort.Strings(c.Extensions)
	x := sort.SearchStrings(c.Extensions, ext)
	return x < len(c.Extensions) && c.Extensions[x] == ext
}
