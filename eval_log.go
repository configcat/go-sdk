package configcat

import (
	"bytes"
	"fmt"
	"strings"
)

var indentBytes = []byte(" ")

const (
	newLineByte byte = '\n'
)

type evalLogBuilder struct {
	builder     strings.Builder
	indentLevel int
}

func (b *evalLogBuilder) resetIndent() *evalLogBuilder {
	b.indentLevel = 0
	return b
}

func (b *evalLogBuilder) incIndent() *evalLogBuilder {
	b.indentLevel++
	return b
}

func (b *evalLogBuilder) decIndent() *evalLogBuilder {
	b.indentLevel--
	return b
}

func (b *evalLogBuilder) newLine() *evalLogBuilder {
	b.builder.WriteByte(newLineByte)
	b.builder.Write(bytes.Repeat(indentBytes, b.indentLevel))
	return b
}

func (b *evalLogBuilder) newLineString(msg string) *evalLogBuilder {
	b.newLine().builder.WriteString(msg)
	return b
}

func (b *evalLogBuilder) append(val interface{}) *evalLogBuilder {
	b.builder.WriteString(fmt.Sprintf("%v", val))
	return b
}
