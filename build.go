package prospect

import (
	"github.com/midbel/toml"
)

type Builder struct {
	Archive
	Context
	Commands []Command `toml:"command"`
	Data     []Data    `toml:"file"`
}

func Build(file, mime string, run func(b Builder, d Data)) error {
	var b Builder
	if err := toml.DecodeFile(file, &b); err != nil {
		return err
	}
	for _, d := range b.Data {
		if d.Mime != mime {
			continue
		}
		run(b, b.Update(d))
	}
	return nil
}

func (b Builder) ExecuteCommands(d Data) ([]Link, error) {
	var ks []Link
	for _, c := range b.Commands {
		x, buf, err := c.Exec(d)
		if err != nil || len(buf) == 0 {
			continue
		}
		k, err := b.CreateFile(x, buf)
		if err != nil {
			continue
		}
		ks = append(ks, k)
	}
	return ks, nil
}
