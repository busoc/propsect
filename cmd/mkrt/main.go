package main

import (
  "flag"
  "fmt"
  "os"

  "github.com/busoc/prospect"
)

func main() {
  flag.Parse()

  err := prospect.Build(flag.Arg(0), collectData, nil)
  if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
  }
}

func collectData(b prospect.Builder, d prospect.Data) {
  filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
    if err != nil || i.IsDir() || !d.Accept(file) {
      return err
    }
    return nil
  })
}
