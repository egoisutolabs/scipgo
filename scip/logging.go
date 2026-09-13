package scip

/*
#include "helpers.h"
*/
import "C"

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
)

// LogLevel distinguishes SCIP's three message channels.
type LogLevel int

// SCIP message channels.
const (
	LogInfo    LogLevel = iota // normal solver output
	LogWarning                 // SCIPwarningMessage
	LogDialog                  // interactive shell output; rarely seen
)

// logSink is the state behind one installed message handler: the user
// callback plus one line buffer per channel, since SCIP delivers messages in
// fragments that may or may not end in '\n'. The whole sink sits behind one
// mutex because SCIPcopy passes the message handler to sub-SCIPs, so during a
// concurrent solve several threads call into the same sink; holding the mutex
// across the callback also preserves SCIP's emission order.
type logSink struct {
	fn  func(level LogLevel, line string)
	mu  sync.Mutex
	buf [3]strings.Builder
}

// write adds one fragment to its channel's buffer, emitting every complete
// line it closes. A panic in the user callback is reported on stderr rather
// than allowed to unwind through C.
func (s *logSink) write(level LogLevel, msg string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "scip: panic in log callback: %v\n", r)
		}
	}()
	s.mu.Lock()
	defer s.mu.Unlock()
	b := &s.buf[level]
	for {
		i := strings.IndexByte(msg, '\n')
		if i < 0 {
			b.WriteString(msg)
			return
		}
		b.WriteString(msg[:i])
		s.emit(level, b.String())
		b.Reset()
		msg = msg[i+1:]
	}
}

// flush emits each channel's buffered partial line, so a model freed with
// output pending loses nothing; the message handler's free callback calls it.
func (s *logSink) flush() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "scip: panic in log callback: %v\n", r)
		}
	}()
	s.mu.Lock()
	defer s.mu.Unlock()
	for level := range s.buf {
		if b := &s.buf[level]; b.Len() > 0 {
			s.emit(LogLevel(level), b.String())
			b.Reset()
		}
	}
}

// emit calls the user callback; the caller holds s.mu.
func (s *logSink) emit(level LogLevel, line string) { s.fn(level, line) }

// logSinks maps the registry id stored in the message handler's data slot to
// its sink, exactly as the plugin registry works: a Go pointer cannot live in
// C memory, so C stores the id instead. The free callback (fired when SCIP
// releases the handler, including from SCIPfree) deletes the entry after
// flushing.
var logSinks = struct {
	mu   sync.Mutex
	next uintptr
	m    map[uintptr]*logSink
}{m: make(map[uintptr]*logSink)}

func putLogSink(s *logSink) uintptr {
	logSinks.mu.Lock()
	defer logSinks.mu.Unlock()
	logSinks.next++
	logSinks.m[logSinks.next] = s
	return logSinks.next
}

func getLogSink(id uintptr) *logSink {
	logSinks.mu.Lock()
	defer logSinks.mu.Unlock()
	return logSinks.m[id]
}

// delLogSink removes the sink and hands it back for a final flush.
func delLogSink(id uintptr) *logSink {
	logSinks.mu.Lock()
	defer logSinks.mu.Unlock()
	s := logSinks.m[id]
	delete(logSinks.m, id)
	return s
}

//export GoMessageInfo
func GoMessageInfo(id C.uintptr_t, msg *C.char) {
	if s := getLogSink(uintptr(id)); s != nil {
		s.write(LogInfo, goString(msg))
	}
}

//export GoMessageWarning
func GoMessageWarning(id C.uintptr_t, msg *C.char) {
	if s := getLogSink(uintptr(id)); s != nil {
		s.write(LogWarning, goString(msg))
	}
}

//export GoMessageDialog
func GoMessageDialog(id C.uintptr_t, msg *C.char) {
	if s := getLogSink(uintptr(id)); s != nil {
		s.write(LogDialog, goString(msg))
	}
}

//export GoMessageHdlrFree
func GoMessageHdlrFree(id C.uintptr_t) (ret C.SCIP_RETCODE) {
	if s := delLogSink(uintptr(id)); s != nil {
		s.flush()
	}
	return C.SCIP_OKAY
}

// TrySetLogFunc routes SCIP's output to fn, one call per complete line
// without the trailing newline. SCIP only installs a message handler while
// the problem is not transformed — the Init and Problem stages, which is
// before the first Solve; after a solve, FreeTransform brings the model back
// and allows the swap again. Routing does not change what SCIP decides to
// print: display/verblevel still applies, and HideOutput silences everything
// before it reaches fn. fn may be called from several threads during a
// concurrent solve and must not call back into the model. Passing nil
// restores SCIP's default stdout handler.
func (m Model) TrySetLogFunc(fn func(level LogLevel, line string)) error {
	defer runtime.KeepAlive(m.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if err := m.guard("SetLogFunc"); err != nil {
		return err
	}
	if !stages(StageInit, StageProblem).has(m.scip.stage()) {
		return m.invalid("SetLogFunc", RetcodeInvalidCall,
			"a message handler can only be installed while the problem is not transformed")
	}
	if fn == nil {
		return m.call("SetLogFunc", C.scipgo_setDefaultMessagehdlr(m.scip.raw))
	}
	id := putLogSink(&logSink{fn: fn})
	if err := m.call("SetLogFunc", C.scipgo_setMessagehdlr(m.scip.raw, C.uintptr_t(id))); err != nil {
		delLogSink(id) // the handler was not installed; drop the sink
		return err
	}
	return nil
}

// SetLogFunc routes SCIP's output to fn; see TrySetLogFunc. It panics on
// failure.
func (m Model) SetLogFunc(fn func(level LogLevel, line string)) Model {
	must(m.TrySetLogFunc(fn))
	return m
}

// SetLogWriter routes SCIP's output to w, newline-terminated; see
// TrySetLogFunc for the staging and threading rules.
func (m Model) SetLogWriter(w io.Writer) Model {
	return m.SetLogFunc(func(_ LogLevel, line string) {
		fmt.Fprintln(w, line)
	})
}

// SetLogger routes SCIP's output to logger: LogInfo and LogDialog lines
// become Info records and LogWarning lines Warn records, with the line as the
// message; see TrySetLogFunc for the staging and threading rules.
func (m Model) SetLogger(logger *slog.Logger) Model {
	return m.SetLogFunc(func(level LogLevel, line string) {
		switch level {
		case LogWarning:
			logger.Warn(line)
		default:
			logger.Info(line)
		}
	})
}

// errLog holds the process-global error sink. Error messages are not part of
// the message handler: SCIPerrorMessage goes through one static printing hook
// for the whole process, so this setting is package-level, not per model.
// Fragments are passed through as they arrive, because there is no hook at
// which a partial error line could be flushed.
var errLog struct {
	mu sync.Mutex
	fn func(line string)
}

//export GoErrorPrinting
func GoErrorPrinting(msg *C.char) {
	errLog.mu.Lock()
	fn := errLog.fn
	errLog.mu.Unlock()
	if fn != nil {
		fn(goString(msg))
	}
}

// SetErrorLogFunc routes SCIP's error messages (SCIPerrorMessage), which are
// global to the process, to fn. Unlike the message channels, error messages
// arrive as fragments and are passed through unchanged, without buffering.
// Passing nil restores the default (stderr).
func SetErrorLogFunc(fn func(line string)) {
	errLog.mu.Lock()
	errLog.fn = fn
	errLog.mu.Unlock()
	if fn != nil {
		C.scipgo_setErrorPrinting()
	} else {
		C.scipgo_setErrorPrintingDefault()
	}
}
