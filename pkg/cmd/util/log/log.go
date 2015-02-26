package log

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/k0kubun/colorstring"
	"github.com/openshift/origin/pkg/cmd/util"
)

const (
	FATAL_EXIT_CODE = 255

	COLOR_INFO    = ""
	COLOR_SUCCESS = "[green]"
	COLOR_WARNING = "[yellow]"
	COLOR_ERROR   = "[red]"
)

// A logger implementation with signature compatible with "github.com/golang/glog", but
// specifically targeted for pretty-printing for end users. Falls back to standard glog.
// Loosely coupled to glog, some features are not implemented and some new ones were added.

// Info logs to the INFO log.
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Info(args ...interface{}) {
	log(infoln, glog.Info, args...)
}

// Infoln logs to the INFO log.
// Arguments are handled in the manner of fmt.Println; a newline is appended if missing.
func Infoln(args ...interface{}) {
	log(infoln, glog.Infoln, args...)
}

// Infof logs to the INFO log.
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Infof(format string, args ...interface{}) {
	logf(infof, glog.Infof, format, args...)
}

// Success logs to the INFO log.
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Success(args ...interface{}) {
	log(successln, glog.Info, args...)
}

// Successln logs to the INFO log.
// Arguments are handled in the manner of fmt.Println; a newline is appended if missing.
func Successln(args ...interface{}) {
	log(successln, glog.Infoln, args...)
}

// Successf logs to the INFO log.
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Successf(format string, args ...interface{}) {
	logf(successf, glog.Infof, format, args...)
}

// Warning logs to the WARNING and INFO logs.
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Warning(args ...interface{}) {
	log(warnln, glog.Warning, args...)
}

// Warningln logs to the WARNING and INFO logs.
// Arguments are handled in the manner of fmt.Println; a newline is appended if missing.
func Warningln(args ...interface{}) {
	log(warnln, glog.Warningln, args...)
}

// Warningf logs to the WARNING and INFO logs.
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Warningf(format string, args ...interface{}) {
	logf(warnf, glog.Warningf, format, args...)
}

// Error logs to the ERROR, WARNING, and INFO logs.
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Error(args ...interface{}) {
	log(errorln, glog.Error, args...)
}

// Errorln logs to the ERROR, WARNING, and INFO logs.
// Arguments are handled in the manner of fmt.Println; a newline is appended if missing.
func Errorln(args ...interface{}) {
	log(errorln, glog.Errorln, args...)
}

// Errorf logs to the ERROR, WARNING, and INFO logs.
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Errorf(format string, args ...interface{}) {
	logf(errorf, glog.Errorf, format, args...)
}

// Fatal logs to the FATAL, ERROR, WARNING, and INFO logs,
// including a stack trace of all running goroutines, then calls os.Exit(255).
// Arguments are handled in the manner of fmt.Print; a newline is appended if missing.
func Fatal(args ...interface{}) {
	log(fatalln, glog.Fatal, args...)
}

// Fatalln logs to the FATAL, ERROR, WARNING, and INFO logs,
// including a stack trace of all running goroutines, then calls os.Exit(255).
// Arguments are handled in the manner of fmt.Println; a newline is appended if missing.
func Fatalln(args ...interface{}) {
	log(fatalln, glog.Fatalln, args...)
}

// Fatalf logs to the FATAL, ERROR, WARNING, and INFO logs,
// including a stack trace of all running goroutines, then calls os.Exit(255).
// Arguments are handled in the manner of fmt.Printf; a newline is appended if missing.
func Fatalf(format string, args ...interface{}) {
	logf(fatalf, glog.Fatalf, format, args...)
}

// FatalDepth acts as Fatal but uses depth to determine which call frame to log.
// FatalDepth(0, "msg") is the same as Fatal("msg").
func FatalDepth(depth int, args ...interface{}) {
	log(fatalln, func(args ...interface{}) {
		glog.FatalDepth(depth, args...)
	}, args...)
}

// logging with level specified is always raw
func V(level glog.Level) glog.Verbose {
	return glog.V(level)
}

func log(onTerminal func(args ...interface{}), fallback func(args ...interface{}), args ...interface{}) {
	out := os.Stdout
	if util.IsTerminal(out) {
		onTerminal(args...)
	} else {
		fallback(args...)
	}
}

func logf(onTerminal func(format string, args ...interface{}), fallback func(format string, args ...interface{}), format string, args ...interface{}) {
	out := os.Stdout
	if util.IsTerminal(out) {
		onTerminal(format, args...)
	} else {
		fallback(format, args...)
	}
}

func infoln(args ...interface{}) {
	println(COLOR_INFO, args...)
}

func infof(format string, args ...interface{}) {
	printf(COLOR_INFO, format, args...)
}

func successln(args ...interface{}) {
	println(COLOR_SUCCESS, args...)
}

func successf(format string, args ...interface{}) {
	printf(COLOR_SUCCESS, format, args...)
}

func warnln(args ...interface{}) {
	println(COLOR_WARNING, args...)
}

func warnf(format string, args ...interface{}) {
	printf(COLOR_WARNING, format, args...)
}

func errorln(args ...interface{}) {
	println(COLOR_ERROR, args...)
}

func errorf(format string, args ...interface{}) {
	printf(COLOR_ERROR, format, args...)
}

func fatalln(args ...interface{}) {
	errorln(args...)
	os.Exit(FATAL_EXIT_CODE)
}

func fatalf(format string, args ...interface{}) {
	errorf(format, args...)
	os.Exit(FATAL_EXIT_CODE)
}

func println(color string, args ...interface{}) {
	s := fmt.Sprint(args...)
	if len(color) > 0 {
		s = colorstring.Color(color + s)
	}
	fmt.Println(s)
}

func printf(color string, format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	if len(color) > 0 {
		s = colorstring.Color(color + s)
	}
	fmt.Println(s)
}
