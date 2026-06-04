package output

import (
	"fmt"
	"io"
	"os"
	"sync/atomic"
)

const (
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiBlue   = "\033[34m"
	ansiCyan   = "\033[36m"
	ansiReset  = "\033[0m"
)

var debugEnabled atomic.Bool

func SetDebug(enabled bool) {
	debugEnabled.Store(enabled)
}

func Info(format string, args ...any) {
	logf(os.Stdout, ansiBlue, "INFO", format, args...)
}

func Debug(format string, args ...any) {
	if !debugEnabled.Load() {
		return
	}
	logf(os.Stdout, ansiCyan, "DEBUG", format, args...)
}

func Success(format string, args ...any) {
	logf(os.Stdout, ansiGreen, "SUCCESS", format, args...)
}

func Warning(format string, args ...any) {
	logf(os.Stdout, ansiYellow, "WARNING", format, args...)
}

func Error(format string, args ...any) {
	logf(os.Stderr, ansiRed, "ERROR", format, args...)
}

func logf(out io.Writer, color, level, format string, args ...any) {
	_, _ = fmt.Fprintf(out, color+"[%s]"+format+ansiReset+"\n", append([]any{level}, args...)...)
}
