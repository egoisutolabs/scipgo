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
	"weak"
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
// line it closes. The buffer is reset before the callback runs, so a panic
// in the user callback for one line cannot corrupt the next, and the panic
// is contained to that line: the remaining lines of a multi-line fragment
// are still delivered.
func (s *logSink) write(level LogLevel, msg string) {
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
		line := b.String()
		b.Reset()
		s.emit(level, line)
		msg = msg[i+1:]
	}
}

// flush emits each channel's buffered partial line, so a model freed with
// output pending loses nothing; the message handler's free callback calls it.
func (s *logSink) flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for level := range s.buf {
		if b := &s.buf[level]; b.Len() > 0 {
			line := b.String()
			b.Reset()
			s.emit(LogLevel(level), line)
		}
	}
}

// emit calls the user callback; the caller holds s.mu. A panic is reported
// on stderr rather than allowed to unwind through C, and contained to this
// line, so later lines are unaffected.
func (s *logSink) emit(level LogLevel, line string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "scip: panic in log callback: %v\n", r)
		}
	}()
	s.fn(level, line)
}

// logSinks maps the registry id stored in the message handler's data slot to
// its sink, exactly as the plugin registry works: a Go pointer cannot live in
// C memory, so C stores the id instead. The registry holds the sink only
// weakly — the owning Scip holds the strong reference — because a callback
// that captures its Model must not root the model through an immortal global
// map: that would stop the finalizer from ever running SCIPfree, and the
// free callback (fired when SCIP releases the handler, including from
// SCIPfree) deletes the entry.
var logSinks = struct {
	mu   sync.Mutex
	next uintptr
	m    map[uintptr]weak.Pointer[logSink]
}{m: make(map[uintptr]weak.Pointer[logSink])}

func putLogSink(s *logSink) uintptr {
	logSinks.mu.Lock()
	defer logSinks.mu.Unlock()
	logSinks.next++
	logSinks.m[logSinks.next] = weak.Make(s)
	return logSinks.next
}

func getLogSink(id uintptr) *logSink {
	logSinks.mu.Lock()
	defer logSinks.mu.Unlock()
	return logSinks.m[id].Value()
}

// delLogSink removes the sink and hands it back for a final flush, if it is
// still alive.
func delLogSink(id uintptr) (*logSink, bool) {
	logSinks.mu.Lock()
	defer logSinks.mu.Unlock()
	wp := logSinks.m[id]
	delete(logSinks.m, id)
	return wp.Value(), true
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
	if s, ok := delLogSink(uintptr(id)); ok {
		s.flush()
	}
	return C.SCIP_OKAY
}

// trySetLog is the one installation path behind SetLogFunc and both
// adapters, so the staging rule is implemented once; op names the public
// method the caller used, which is what errors carry.
func (m Model) trySetLog(op string, fn func(level LogLevel, line string)) error {
	defer runtime.KeepAlive(m.scip.root()) // pin the strong instance, not a weak wrapper, until the C call returns
	if err := m.guard(op); err != nil {
		return err
	}
	if !stages(StageInit, StageProblem).has(m.scip.stage()) {
		return m.invalid(op, RetcodeInvalidCall,
			"a message handler can only be installed while the problem is not transformed")
	}
	if fn == nil {
		m.scip.logSink = nil
		return m.call(op, C.scipgo_setDefaultMessagehdlr(m.scip.raw))
	}
	sink := &logSink{fn: fn}
	id := putLogSink(sink)
	if err := m.call(op, C.scipgo_setMessagehdlr(m.scip.raw, C.uintptr_t(id))); err != nil {
		delLogSink(id) // the handler was not installed; drop the sink
		return err
	}
	// The owning instance keeps the sink alive; the registry holds it only
	// weakly, so a callback capturing its Model cannot root the model and
	// block its finalizer.
	if r := m.scip.root(); r != nil {
		r.logSink = sink
	}
	m.scip.logSink = sink
	return nil
}

// TrySetLogFunc routes SCIP's output to fn, one call per complete line
// without the trailing newline. SCIP only installs a message handler while
// the problem is not transformed — the Init and Problem stages, which is
// before the first Solve; after a solve, FreeTransform brings the model back
// and allows the swap again. Routing does not change what SCIP decides to
// print: display/verblevel still applies, and HideOutput silences everything
// before it reaches fn. fn may be called from several threads during a
// concurrent solve. The sink's lock is held while fn runs, so fn must not
// call into SCIP — through any Model — nor swap the sink: a nested message
// or error would wait for that lock forever. Concurrent solves share the one
// handler, and SCIP's callbacks carry no worker identity, so lines emitted
// from different workers can interleave at fragment granularity — one
// emitted line may be assembled from two workers' fragments; SCIP's own
// default stdout handler mixes them the same way. Passing nil restores
// SCIP's default stdout handler.
func (m Model) TrySetLogFunc(fn func(level LogLevel, line string)) error {
	return m.trySetLog("SetLogFunc", fn)
}

// SetLogFunc routes SCIP's output to fn; see TrySetLogFunc. It panics on
// failure.
func (m Model) SetLogFunc(fn func(level LogLevel, line string)) Model {
	must(m.TrySetLogFunc(fn))
	return m
}

// TrySetLogWriter routes SCIP's output to w, newline-terminated; see
// TrySetLogFunc for the staging and threading rules. It returns an error
// rather than panicking when the sink cannot be installed.
func (m Model) TrySetLogWriter(w io.Writer) error {
	if isNilInterface(w) {
		return m.invalid("SetLogWriter", RetcodeInvalidData, "nil io.Writer")
	}
	return m.trySetLog("SetLogWriter", func(_ LogLevel, line string) {
		fmt.Fprintln(w, line)
	})
}

// SetLogWriter routes SCIP's output to w; see TrySetLogWriter. It panics on
// failure.
func (m Model) SetLogWriter(w io.Writer) Model {
	must(m.TrySetLogWriter(w))
	return m
}

// TrySetLogger routes SCIP's output to logger: LogInfo and LogDialog lines
// become Info records and LogWarning lines Warn records, with the line as
// the message; see TrySetLogFunc for the staging and threading rules. It
// returns an error rather than panicking when the sink cannot be installed.
func (m Model) TrySetLogger(logger *slog.Logger) error {
	if logger == nil {
		return m.invalid("SetLogger", RetcodeInvalidData, "nil *slog.Logger")
	}
	return m.trySetLog("SetLogger", func(level LogLevel, line string) {
		switch level {
		case LogWarning:
			logger.Warn(line)
		default:
			logger.Info(line)
		}
	})
}

// SetLogger routes SCIP's output to logger; see TrySetLogger. It panics on
// failure.
func (m Model) SetLogger(logger *slog.Logger) Model {
	must(m.TrySetLogger(logger))
	return m
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
	// The mutex is held across the callback: calls are serialized like the
	// per-model sink's, and the panic recovery boundary applies here too —
	// this runs under SCIP's C frames.
	errLog.mu.Lock()
	defer errLog.mu.Unlock()
	if errLog.fn == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "scip: panic in error log callback: %v\n", r)
		}
	}()
	errLog.fn(goString(msg))
}

// SetErrorLogFunc routes SCIP's error messages (SCIPerrorMessage), which are
// global to the process, to fn. Unlike the message channels, error messages
// arrive as fragments and are passed through unchanged, without buffering.
// Calls to fn are serialized against each other and against SetErrorLogFunc
// itself, and the serialization lock is held while fn runs — so fn must not
// call into SCIP, through any Model, nor call SetErrorLogFunc: a nested
// error message would wait for that lock forever. Passing nil restores the
// default (stderr).
func SetErrorLogFunc(fn func(line string)) {
	// The whole transition — Go state and C hook — happens under one lock,
	// so enable/disable cannot interleave into an installed hook with no
	// function, or a function with no hook.
	errLog.mu.Lock()
	defer errLog.mu.Unlock()
	errLog.fn = fn
	if fn != nil {
		C.scipgo_setErrorPrinting()
	} else {
		C.scipgo_setErrorPrintingDefault()
	}
}
