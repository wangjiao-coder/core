package logging

import (
	"bytes"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/stretchr/testify/assert"
)

func TestWithLevel(t *testing.T) {
	var buf bytes.Buffer
	l := log.NewLogfmtLogger(&buf)
	ll := WithLevel(l)
	ll.Debug("hi")
	// ensure the caller depth is correct
	assert.Contains(t, buf.String(), "caller=log_test.go")
	assert.Contains(t, buf.String(), "level=debug")

	ll.Debugw("foo", "bar", "baz")
	assert.Contains(t, buf.String(), "bar=baz")

	ll.Debugf("foo%d", 1)
	assert.Contains(t, buf.String(), "foo1")
}

func TestLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	l := log.NewLogfmtLogger(&buf)
	l = level.NewFilter(l, LevelFilter("error"))
	WithLevel(l).Debug("hi")
	// ensure the caller depth is correct
	assert.NotContains(t, buf.String(), "caller=log_test.go")
}

func TestNewLogger(t *testing.T) {
	_ = NewLogger("logfmt")
}
