package trace

import (
	"fmt"
	"log"
	"os"
	"time"
  "path/filepath"

	"github.com/busoc/prospect"
)

type Tracer struct {
	now    time.Time
	logger *log.Logger

	err   uint64
	files uint64
	size  float64
	when  time.Time
}

func New(name string) *Tracer {
	name = fmt.Sprintf("[%s] ", name)
	t := Tracer{
		logger: log.New(os.Stdout, name, log.LstdFlags),
		when:   time.Now(),
	}
	return &t
}

func (t *Tracer) Start(file string) {
	t.now = time.Now()
	t.files++
	t.Trace("start processing %s", file)
}

func (t *Tracer) Summarize() {
	elapsed := time.Since(t.when)
	t.Trace("%d files processed (%s - %.0f - %d errors)", t.files, elapsed, t.size, t.err)
}

func (t *Tracer) Done(file string, d prospect.Data) {
  var (
    elapsed = time.Since(t.now)
    archive = filepath.Join(d.Resolve(), filepath.Base(d.File))
  )
	t.size += float64(d.Size)
	t.Trace("done processing %s -> %s (%d, %s)", file, archive, d.Size, elapsed)
}

func (t *Tracer) Error(file string, err error) {
	t.err++
	t.Trace("error while processing %s: %s", err)
}

func (t *Tracer) Trace(msg string, args ...interface{}) {
	t.logger.Printf(msg, args...)
}
