package prospect

import (
	"fmt"
)

type BytesCounter struct {
	size int64
}

func NewCounter() *BytesCounter {
	var b BytesCounter
	return &b
}

func (b *BytesCounter) Write(bs []byte) (int, error) {
	n := len(bs)
	b.size += int64(n)
	return n, nil
}

func (b *BytesCounter) Size() int64 {
	return b.size
}

func (b *BytesCounter) AsParameter() Parameter {
	return MakeParameter(FileSize, fmt.Sprintf("%d", b.size))
}
