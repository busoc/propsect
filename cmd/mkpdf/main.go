package main

import (
  "flag"
  "fmt"
  "path/filepath"
  "os"

  "github.com/busoc/prospect"
  "github.com/busoc/prospect/cmd/internal/trace"
  "github.com/midbel/mime"
  "github.com/midbel/pdf"
)

const (
	MainType   = "application"
	SubType    = "pdf"
  fileAuthor = "file.author"
  fileSubject = "file.subject"
  fileTitle = "file.title"
  fileKeyword = "file.%d.keyword"
)

func main() {
	flag.Parse()

	accept := func(d prospect.Data) bool {
		m, err := mime.Parse(d.Mime)
		if err != nil {
			return false
		}
		return m.MainType == MainType && m.SubType == SubType
	}
	err := prospect.Build(flag.Arg(0), collectData, accept)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func collectData(b prospect.Builder, d prospect.Data) {
	tracer := trace.New("mkpdf")
	defer tracer.Summarize()
	filepath.Walk(d.File, func(file string, i os.FileInfo, err error) error {
		if err != nil || i.IsDir() || !d.Accept(file) {
			return err
		}
		dat := d.Clone()

		tracer.Start(file)

		if dat, err = processData(dat, file); err != nil {
			tracer.Error(file, err)
			return nil
		}
		if err := b.Store(dat); err != nil {
			tracer.Error(file, err)
		}
		tracer.Done(file, dat)
		return nil
	})
}

func processData(d prospect.Data, file string) (prospect.Data, error) {
	if err := prospect.ReadFile(&d, file); err != nil {
		return d, err
	}
	return readFile(d)
}

func readFile(d prospect.Data) (prospect.Data, error) {
  doc, err := pdf.Open(d.File)
  if err != nil {
    return d, err
  }
  defer doc.Close()

  info := doc.GetDocumentInfo()
  d.AcqTime = info.Created
  d.ModTime = info.Modified

  d.Register(fileTitle, info.Title)
  d.Register(fileSubject, info.Subject)
  d.Register(fileAuthor, info.Author)
  for i := range info.Keywords {
    d.Register(fmt.Sprintf(fileKeyword, i), info.Keywords[i])
  }
  return d, nil
}
