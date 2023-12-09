package configcat

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

var indentBytes = []byte("  ")

const (
	newLineByte         byte = '\n'
	stringListMaxLength      = 10
)

type evalLogBuilder struct {
	builder     strings.Builder
	indentLevel int
	user        User
}

func (b *evalLogBuilder) userAsString() string {
	return fmt.Sprintf("%#v", b.user)
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

func (b *evalLogBuilder) appendUserCondition(comparisonAttribute string, op Comparator, comparisonValue interface{}) *evalLogBuilder {
	b.append(fmt.Sprintf("User.%s %s ", comparisonAttribute, op.String()))
	if comparisonValue == nil {
		return b.append("<invalid value>")
	}
	switch val := comparisonValue.(type) {
	case float64:
		if op.IsDateTime() {
			t := time.UnixMilli(int64(val) * 1000)
			return b.append(fmt.Sprintf("'%.0f' (%s)", val, t.UTC()))
		} else {
			return b.append(fmt.Sprintf("'%g'", val))
		}
	case string:
		var res string
		if op.IsSensitive() {
			res = "<hashed value>"
		} else {
			res = val
		}
		return b.append(fmt.Sprintf("'%s'", res))
	case []string:
		if op.IsSensitive() {
			var valText string
			if len(val) > 1 {
				valText = "values"
			} else {
				valText = "value"
			}
			return b.append(fmt.Sprintf("[<%d hashed %s>]", len(val), valText))
		} else {
			length := len(val)
			var valText string
			if length-stringListMaxLength > 1 {
				valText = "values"
			} else {
				valText = "value"
			}
			var res string
			limit := length
			if limit > stringListMaxLength {
				limit = stringListMaxLength
			}
			for i, item := range val {
				res += "'" + item + "'"

				if i < limit-1 {
					res += ", "
				} else if length > stringListMaxLength {
					res += fmt.Sprintf(", ... <%d more %s>", length-stringListMaxLength, valText)
					break
				}
			}
			return b.append(fmt.Sprintf("[%s]", res))
		}
	}
	return b
}
