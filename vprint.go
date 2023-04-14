package main

// Copyright (c) 2023 Jason E. Aten, Ph.D.
// License: MIT; see LICENSE file.

import (
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"sync"
	"time"

	"4d63.com/tz"
)

// for tons of debug output
var Verbose bool = false
var VerboseVerbose bool = false

var MyPid = os.Getpid()
var ShowPid bool

var UtcTz *time.Location
var NYC *time.Location
var Chicago *time.Location
var Frankfurt *time.Location
var London *time.Location
var IST *time.Location // Indian Standard Time
var Halifax *time.Location

func init() {
	initTimezonesEtc()
}

func initTimezonesEtc() {

	// do this is ~/.bashrc so we get the default.
	os.Setenv("TZ", "America/Chicago")

	var err error
	UtcTz, err = tz.LoadLocation("UTC")
	panicOn(err)
	NYC, err = tz.LoadLocation("America/New_York")
	stopOn(err)
	Chicago, err = tz.LoadLocation("America/Chicago")
	stopOn(err)
	Frankfurt, err = tz.LoadLocation("Europe/Berlin")
	stopOn(err)
	IST, err = tz.LoadLocation("Asia/Kolkata") // Indian Standard Time; UTC + 05:30
	stopOn(err)
	Halifax, err = tz.LoadLocation("America/Halifax")
	stopOn(err)
	London, err = tz.LoadLocation("Europe/London")
	stopOn(err)
}

func P(format string, a ...interface{}) {
	if Verbose {
		TSPrintf(format, a...)
	}
}

func PP(format string, a ...interface{}) {
	if VerboseVerbose {
		TSPrintf(format, a...)
	}
}

// useful during git bisect
var ForceQuiet = false

func VV(format string, a ...interface{}) {
	if !ForceQuiet {
		TSPrintf(format, a...)
	}
}

func AlwaysPrintf(format string, a ...interface{}) {
	TSPrintf(format, a...)
}

var vv = VV

// without the file/line, otherwise the same as PP
func PPP(format string, a ...interface{}) {
	if VerboseVerbose {
		Printf("\n%s ", ts())
		Printf(format+"\n", a...)
	}
}

func PB(w io.Writer, format string, a ...interface{}) {
	if Verbose {
		fmt.Fprintf(w, "\n"+format+"\n", a...)
	}
}

var tsPrintfMut sync.Mutex

// time-stamped printf
func TSPrintf(format string, a ...interface{}) {
	tsPrintfMut.Lock()
	if ShowPid {
		Printf("\n%s [pid %v] %s ", FileLine(3), MyPid, ts())
	} else {
		Printf("\n%s %s ", FileLine(3), ts())
	}
	Printf(format+"\n", a...)
	tsPrintfMut.Unlock()
}

// get timestamp for logging purposes
func ts() string {
	return time.Now().In(Chicago).Format("2006-01-02 15:04:05.999 -0700 MST")
	//return time.Now().In(NYC).Format("2006-01-02 15:04:05.999 -0700 MST")
}

// so we can multi write easily, use our own printf
var OurStdout io.Writer = os.Stdout

// Printf formats according to a format specifier and writes to standard output.
// It returns the number of bytes written and any write error encountered.
func Printf(format string, a ...interface{}) (n int, err error) {
	return fmt.Fprintf(OurStdout, format, a...)
}

func FileLine(depth int) string {
	_, fileName, fileLine, ok := runtime.Caller(depth)
	var s string
	if ok {
		s = fmt.Sprintf("%s:%d", path.Base(fileName), fileLine)
	} else {
		s = ""
	}
	return s
}

func p(format string, a ...interface{}) {
	if Verbose {
		TSPrintf(format, a...)
	}
}

var pp = PP

func pbb(w io.Writer, format string, a ...interface{}) {
	if Verbose {
		fmt.Fprintf(w, "\n"+format+"\n", a...)
	}
}

// quieted for now, uncomment below to display
func VPrintf(format string, a ...interface{}) (n int, err error) {
	//return fmt.Fprintf(OurStdout, format, a...)
	return
}

func QPrintf(format string, a ...interface{}) (n int, err error) {
	//return fmt.Fprintf(OurStdout, format, a...)
	return
}

func Caller(upStack int) string {
	// elide ourself and runtime.Callers
	target := upStack + 2

	pc := make([]uintptr, target+2)
	n := runtime.Callers(0, pc)

	f := runtime.Frame{Function: "unknown"}
	if n > 0 {
		frames := runtime.CallersFrames(pc[:n])
		for i := 0; i <= target; i++ {
			contender, more := frames.Next()
			if i == target {
				f = contender
			}
			if !more {
				break
			}
		}
	}
	return f.Function
}

func stopOn(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", FileLine(2), err.Error())
	os.Exit(1)
}

func panicOn(err error) {
	if err != nil {
		panicOn(err)
	}
}
