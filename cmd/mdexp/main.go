package main

import (
  "fmt"
  "flag"
  "os"
  "io"
  "path/filepath"

  "github.com/midbel/toml"
  "github.com/busoc/prospect"
)

func main() {
  dir := flag.String("d", "", "output directory")
  flag.Parse()

  var meta prospect.Meta
  if err := toml.DecodeFile(flag.Arg(0), &meta); err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(2)
  }

  var w io.Writer = os.Stdout
  if *dir != "" {
    if err := os.MkdirAll(*dir, 0755); err != nil {
      fmt.Fprintln(os.Stderr, err)
      os.Exit(1)
    }
    base := fmt.Sprintf("MD_EXP_%s.xml", meta.Accr)
    f, err := os.Create(filepath.Join(*dir, base))
    if err != nil {
      fmt.Fprintln(os.Stderr, err)
      os.Exit(1)
    }
    defer f.Close()
    w = f
  }
  if err := prospect.EncodeMeta(w, meta); err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
  }
}
