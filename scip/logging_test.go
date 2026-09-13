package scip

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// captureFd1 redirects file descriptor 1 — which C's stdout writes to, not
// just Go's os.Stdout — to a pipe for the duration of fn, and returns what
// was written.
func captureFd1(t *testing.T, fn func()) string {
	t.Helper()
	saved, err := syscall.Dup(1)
	if err != nil {
		t.Fatalf("dup: %v", err)
	}
	defer func() {
		if err := syscall.Close(saved); err != nil && !os.IsNotExist(err) {
			t.Fatalf("close saved fd: %v", err)
		}
	}()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if err := syscall.Dup2(int(w.Fd()), 1); err != nil {
		t.Fatalf("dup2: %v", err)
	}
	fn()
	if err := syscall.Dup2(saved, 1); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read: %v", err)
	}
	return buf.String()
}

// TestSetLogWriterCapturesSolve checks the headline: with a sink installed,
// the solve output lands in the sink and nothing leaks to process stdout.
func TestSetLogWriterCapturesSolve(t *testing.T) {
	var buf bytes.Buffer
	stdout := captureFd1(t, func() {
		model := NewModel().SetLogWriter(&buf).IncludeDefaultPlugins()
		if _, err := model.ReadProb(testFile("simple.mps")); err != nil {
			t.Errorf("read: %v", err)
			return
		}
		model.Solve()
	})
	if !strings.Contains(buf.String(), "original problem") {
		t.Errorf("sink missed the problem line; got %d bytes", buf.Len())
	}
	if !strings.Contains(buf.String(), "SCIP Status") {
		t.Errorf("sink missed the SCIP Status line")
	}
	if stdout != "" {
		t.Errorf("%d bytes leaked to process stdout: %q", len(stdout), stdout[:min(len(stdout), 200)])
	}
}

// TestLogLinesAreWhole checks fragments are buffered to whole lines: no
// emitted line contains a newline, and the display header — assembled from
// several messages — arrives as one record.
func TestLogLinesAreWhole(t *testing.T) {
	var mu sync.Mutex
	var lines []string
	model := NewModel().SetLogFunc(func(_ LogLevel, line string) {
		mu.Lock()
		lines = append(lines, line)
		mu.Unlock()
	}).IncludeDefaultPlugins()
	if _, err := model.ReadProb(testFile("simple.mps")); err != nil {
		t.Fatalf("read: %v", err)
	}
	model.SetPresolving(ParamSettingOff).Solve()
	mu.Lock()
	defer mu.Unlock()
	if len(lines) < 5 {
		t.Fatalf("only %d lines captured", len(lines))
	}
	header := false
	for _, l := range lines {
		if strings.Contains(l, "\n") {
			t.Errorf("emitted line contains a newline: %q", l)
		}
		if strings.Contains(l, " time | node") && strings.Contains(l, "dualbound") {
			header = true
		}
	}
	if !header {
		t.Errorf("display header did not arrive as one record")
	}
}

// TestHideOutputSilencesSink checks routing does not bypass verbosity: with
// display/verblevel at zero nothing reaches the sink.
func TestHideOutputSilencesSink(t *testing.T) {
	var buf bytes.Buffer
	model := NewModel().SetLogWriter(&buf).IncludeDefaultPlugins().HideOutput()
	if _, err := model.ReadProb(testFile("simple.mps")); err != nil {
		t.Fatalf("read: %v", err)
	}
	model.Solve()
	if buf.Len() != 0 {
		t.Errorf("sink received %d bytes despite HideOutput", buf.Len())
	}
}

// recordingHandler collects slog records for SetLogger assertions.
type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }

// TestSetLogger checks the slog mapping: info lines become Info records.
// None of the bundled models provokes SCIPwarningMessage, so the Warn
// mapping is checked against the installed sink directly.
func TestSetLogger(t *testing.T) {
	h := &recordingHandler{}
	logger := slog.New(h)
	model := NewModel().SetLogger(logger).IncludeDefaultPlugins()
	if _, err := model.ReadProb(testFile("simple.mps")); err != nil {
		t.Fatalf("read: %v", err)
	}
	model.Solve()

	h.mu.Lock()
	if len(h.records) == 0 {
		t.Fatal("no slog records emitted")
	}
	for _, r := range h.records {
		if r.Level != slog.LevelInfo {
			t.Errorf("solve output mapped to %v, want Info", r.Level)
		}
	}
	h.mu.Unlock()

	// The installed sink, given a warning line, must map to Warn. Earlier
	// tests leave their sinks registered, so the probe goes to all of them
	// and the marker is matched by message: only ours writes to this
	// logger.
	logSinks.mu.Lock()
	sinks := make([]*logSink, 0, len(logSinks.m))
	for _, wp := range logSinks.m {
		if s := wp.Value(); s != nil {
			sinks = append(sinks, s)
		}
	}
	logSinks.mu.Unlock()
	if len(sinks) == 0 {
		t.Fatal("no sink registered")
	}
	for _, sink := range sinks {
		sink.write(LogWarning, "worse things have happened\n")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	found := false
	for _, r := range h.records {
		if r.Message == "worse things have happened" && r.Level == slog.LevelWarn {
			found = true
		}
	}
	if !found {
		t.Errorf("warning line not mapped to a Warn record")
	}
}

// TestWriteStatsJSONNotRouted checks the file hint: statistics written to a
// real FILE* go to that file, not through the sink.
func TestWriteStatsJSONNotRouted(t *testing.T) {
	var buf bytes.Buffer
	model := NewModel().SetLogWriter(&buf).IncludeDefaultPlugins()
	if _, err := model.ReadProb(testFile("simple.mps")); err != nil {
		t.Fatalf("read: %v", err)
	}
	model.Solve()
	buf.Reset()

	path := filepath.Join(t.TempDir(), "stats.json")
	if err := model.WriteStatsJSON(path); err != nil {
		t.Fatalf("WriteStatsJSON: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		t.Fatalf("stats file not written: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("stats file is not JSON: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("stats leaked into the sink: %q", buf.String()[:min(buf.Len(), 200)])
	}
}

// TestConcurrentSolveWithSink checks the sink is safe under a concurrent
// solve; run with -race. SCIPcopy passes the message handler to the workers
// it spawns, so the sink can be called from several threads at once. Whether
// SCIP spawns workers at all is its own decision (parallel/maxnthreads,
// memory limits, models presolve resolves); on this build gen-ip054 with two
// threads runs the solve through the master, so the assertion is on output
// captured and race cleanliness, not on worker lines specifically.
func TestConcurrentSolveWithSink(t *testing.T) {
	var mu sync.Mutex
	var lines []string
	model := hardTestModel(t).SetLogFunc(func(_ LogLevel, line string) {
		mu.Lock()
		lines = append(lines, line)
		mu.Unlock()
	}).SetDisplayVerbosity(4)
	model, err := model.SetIntParam("parallel/maxnthreads", 2)
	if err != nil {
		t.Fatal(err)
	}
	if model, err = model.SetRealParam("limits/time", 2); err != nil {
		t.Fatal(err)
	}
	model.SolveConcurrent()
	mu.Lock()
	defer mu.Unlock()
	if len(lines) == 0 {
		t.Error("sink captured nothing during the concurrent solve")
	}
	for _, l := range lines {
		if strings.Contains(l, "\n") {
			t.Errorf("emitted line contains a newline: %q", l)
		}
	}
}

// TestSetLogFuncNilRestoresStdout checks the default handler comes back.
func TestSetLogFuncNilRestoresStdout(t *testing.T) {
	stdout := captureFd1(t, func() {
		model := NewModel().SetLogWriter(io.Discard).SetLogFunc(nil).IncludeDefaultPlugins()
		if _, err := model.ReadProb(testFile("simple.mps")); err != nil {
			t.Errorf("read: %v", err)
			return
		}
		model.Solve()
	})
	if !strings.Contains(stdout, "SCIP Status") {
		t.Errorf("stdout not restored; %d bytes captured", len(stdout))
	}
}

// TestSinkFlush checks partial lines do not vanish: a fragment without a
// trailing newline is emitted by flush, which the message handler's free
// callback calls when SCIP releases it.
func TestSinkFlush(t *testing.T) {
	var mu sync.Mutex
	var got []LogLevel
	var lines []string
	sink := &logSink{fn: func(level LogLevel, line string) {
		mu.Lock()
		got = append(got, level)
		lines = append(lines, line)
		mu.Unlock()
	}}
	sink.write(LogInfo, "complete line\n")
	sink.write(LogWarning, "partial ")
	sink.write(LogWarning, "warning")
	mu.Lock()
	if len(lines) != 1 || lines[0] != "complete line" || got[0] != LogInfo {
		mu.Unlock()
		t.Fatalf("after writes: %v %q", got, lines)
	}
	mu.Unlock()
	sink.flush()
	mu.Lock()
	defer mu.Unlock()
	if len(lines) != 2 || lines[1] != "partial warning" || got[1] != LogWarning {
		t.Fatalf("after flush: %v %q", got, lines)
	}
}

// TestSinkPanicDoesNotCorruptNextLine checks the buffer is cleared before
// the callback runs: a panic on one line must not leave it buffered and
// concatenated onto the next.
func TestSinkPanicDoesNotCorruptNextLine(t *testing.T) {
	var mu sync.Mutex
	var lines []string
	sink := &logSink{fn: func(_ LogLevel, line string) {
		mu.Lock()
		defer mu.Unlock()
		if line == "boom" {
			panic("callback panicked")
		}
		lines = append(lines, line)
	}}
	sink.write(LogInfo, "boom\n")
	sink.write(LogInfo, "next line\n")
	// A multi-line fragment must survive a panic on its first line.
	sink.write(LogInfo, "boom\nsurvivor\n")
	mu.Lock()
	defer mu.Unlock()
	if len(lines) != 2 || lines[0] != "next line" || lines[1] != "survivor" {
		t.Fatalf("lines after panic: %q", lines)
	}
}

// TestSetErrorLogFunc checks the process-global error sink: an unknown
// parameter produces SCIPerrorMessage fragments, passed through unbuffered.
func TestSetErrorLogFunc(t *testing.T) {
	var mu sync.Mutex
	var joined strings.Builder
	SetErrorLogFunc(func(line string) {
		mu.Lock()
		joined.WriteString(line)
		mu.Unlock()
	})
	defer SetErrorLogFunc(nil)
	model := NewModel().IncludeDefaultPlugins()
	if _, err := model.SetIntParam("display/nosuchparam", 1); err == nil {
		t.Fatal("expected an error for the unknown parameter")
	}
	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(joined.String(), "parameter <display/nosuchparam> unknown") {
		t.Errorf("error sink missed the message; got %q", joined.String())
	}
}

// TestErrorLogFuncPanic checks a panicking error sink reports on stderr
// instead of unwinding through SCIP's C frames.
func TestErrorLogFuncPanic(t *testing.T) {
	SetErrorLogFunc(func(string) { panic("error sink panicked") })
	defer SetErrorLogFunc(nil)
	model := NewModel().IncludeDefaultPlugins()
	if _, err := model.SetIntParam("display/nosuchparam", 1); err == nil {
		t.Fatal("expected an error for the unknown parameter")
	}
}

// TestSetLogFuncStageError checks the staging rule: the problem's
// transformed space is off limits, so after a solve the swap is refused —
// FreeTransform brings the model back to the Problem stage and allows it
// again.
func TestSetLogFuncStageError(t *testing.T) {
	model := mustRead(t, NewModel().IncludeDefaultPlugins(), testFile("simple.mps"))
	defer model.Free()
	model.HideOutput().Solve()
	err := model.TrySetLogFunc(func(LogLevel, string) {})
	if err == nil || !strings.Contains(err.Error(), "not transformed") {
		t.Fatalf("want staging error, got %v", err)
	}
	// The adapters have Try forms too, with the same staging rule, and
	// reject nil sinks outright.
	if err := model.TrySetLogWriter(io.Discard); err == nil {
		t.Error("TrySetLogWriter in a transformed stage should fail")
	}
	if err := model.TrySetLogger(slog.New(&recordingHandler{})); err == nil {
		t.Error("TrySetLogger in a transformed stage should fail")
	}
	fresh := NewModel().IncludeDefaultPlugins()
	defer fresh.Free()
	if err := fresh.TrySetLogWriter(nil); err == nil {
		t.Error("TrySetLogWriter(nil) should fail")
	}
	if err := fresh.TrySetLogger(nil); err == nil {
		t.Error("TrySetLogger(nil) should fail")
	}
	// A typed nil writer inside the interface is caught too, instead of
	// installing a sink that panics on the first emitted line.
	var bufPtr *bytes.Buffer
	if err := fresh.TrySetLogWriter(bufPtr); err == nil || !errors.Is(err, RetcodeInvalidData) {
		t.Errorf("TrySetLogWriter(typed nil) = %v, want RetcodeInvalidData", err)
	}
	// Errors carry the adapter's own operation name.
	if e := asError(t, model.TrySetLogWriter(io.Discard)); e.Op != "SetLogWriter" {
		t.Errorf("TrySetLogWriter error Op = %q, want SetLogWriter", e.Op)
	}
	if e := asError(t, model.TrySetLogger(slog.New(&recordingHandler{}))); e.Op != "SetLogger" {
		t.Errorf("TrySetLogger error Op = %q, want SetLogger", e.Op)
	}
}

// TestSetLogFuncNilFlushesPartialLine checks the swap ordering: the old
// handler is released inside the C call that installs the default one, and
// its free callback flushes the old sink, which the model still holds
// strongly at that moment — so a buffered partial line survives the swap.
func TestSetLogFuncNilFlushesPartialLine(t *testing.T) {
	var mu sync.Mutex
	var lines []string
	model := NewModel().IncludeDefaultPlugins()
	model.SetLogFunc(func(_ LogLevel, line string) {
		mu.Lock()
		lines = append(lines, line)
		mu.Unlock()
	})
	model.scip.logSink.write(LogInfo, "partial before the swap")
	model.SetLogFunc(nil)
	mu.Lock()
	defer mu.Unlock()
	if len(lines) != 1 || lines[0] != "partial before the swap" {
		t.Fatalf("lines after swap: %q", lines)
	}
}

// TestDroppedModelDoesNotLeakSink checks the ownership direction: the
// registry holds sinks only weakly, and the model holds the strong
// reference, so the tempting pattern — a callback capturing its own Model —
// cannot root the model through an immortal global map and block its
// finalizer from ever freeing SCIP.
func TestDroppedModelDoesNotLeakSink(t *testing.T) {
	install := func() {
		model := NewModel().IncludeDefaultPlugins()
		model.SetLogFunc(func(_ LogLevel, _ string) { _ = model }) // captures the model
		// model dropped without Free: the finalizer must be able to run,
		// SCIPfree fires the handler's free callback, and the registry
		// entry disappears. The variable is cleared to drop it while the
		// closure keeps a captured reference — leaving it live in the
		// enclosing frame would root the model and test nothing.
		model = Model{}
	}
	install()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		runtime.GC()
		logSinks.mu.Lock()
		n := len(logSinks.m)
		logSinks.mu.Unlock()
		if n == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("sink registry entry survived its dropped model")
}
