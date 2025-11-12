package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

const (
	ansiReset  = "\033[0m"
	ansiGreen  = "\033[32m"
	ansiCyan   = "\033[36m"
	ansiYellow = "\033[33m"
	ansiRed    = "\033[31m"
)

// Level represents a logging severity.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String returns the canonical upper-case label for the log level.
func (lvl Level) String() string {
	switch lvl {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

var levelColors = map[Level]string{
	LevelDebug: ansiCyan,
	LevelInfo:  ansiGreen,
	LevelWarn:  ansiYellow,
	LevelError: ansiRed,
}

const defaultCallDepth = 4

// Config controls the construction of a Logger instance.
type Config struct {
	Output       io.Writer
	DisableColor bool
	Debug        LevelOptions
	Info         LevelOptions
	Warn         LevelOptions
	Error        LevelOptions
	// File, if set, will be appended to and included in outputs when
	// multi-writer behavior is requested.
	File string
	// Stdout when true includes stdout in all level outputs via MultiWriter.
	Stdout bool
	// ErrorToStderr when true ensures Error level also writes to stderr.
	ErrorToStderr bool
}

// LevelOptions configures a single log level.
type LevelOptions struct {
	Enabled *bool
	Flags   *int
	Prefix  string
	Writer  io.Writer
}

// Logger provides leveled, colorized logging suitable for CLI services.
type Logger struct {
	mu       sync.RWMutex
	levels   map[Level]*levelState
	colorize bool
	closers  []io.Closer
}

type levelState struct {
	logger     *log.Logger
	enabled    bool
	basePrefix string
	color      string
}

var std = New(Config{})

// New builds a new Logger based on the provided configuration.
func New(cfg Config) *Logger {
	cfg = cfg.withDefaults()

	// if File is set, attempt to open it for append and include it in
	// per-level multiwriters. Keep the file open for the lifetime of the
	// logger and record it for Close().
	var fileHandle *os.File
	if cfg.File != "" {
		f, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			fileHandle = f
		} else {
			// fallback: log to stderr if file can't be opened
			fmt.Fprintf(os.Stderr, "logger: failed to open file %s: %v\n", cfg.File, err)
		}
	}

	// helper to build a writer for a level
	buildWriter := func(level Level, base io.Writer) io.Writer {
		var writers []io.Writer
		// include stdout if requested
		if cfg.Stdout {
			writers = append(writers, os.Stdout)
		}
		// include file if opened
		if fileHandle != nil {
			writers = append(writers, fileHandle)
		}
		// include stderr for errors if requested
		if level == LevelError && cfg.ErrorToStderr {
			writers = append(writers, os.Stderr)
		}
		// always include the base writer last
		if base != nil {
			writers = append(writers, base)
		}
		if len(writers) == 0 {
			return base
		}
		if len(writers) == 1 {
			return writers[0]
		}
		return io.MultiWriter(writers...)
	}

	// apply multiwriter choices to level options before constructing levelState
	cfg.Debug.Writer = buildWriter(LevelDebug, cfg.Debug.Writer)
	cfg.Info.Writer = buildWriter(LevelInfo, cfg.Info.Writer)
	cfg.Warn.Writer = buildWriter(LevelWarn, cfg.Warn.Writer)
	cfg.Error.Writer = buildWriter(LevelError, cfg.Error.Writer)

	l := &Logger{
		levels:   make(map[Level]*levelState, 4),
		colorize: !cfg.DisableColor,
	}
	if fileHandle != nil {
		// record file handle to close later
		l.closers = append(l.closers, fileHandle)
	}

	l.levels[LevelDebug] = newLevelState(LevelDebug, cfg.Debug, l.colorize)
	l.levels[LevelInfo] = newLevelState(LevelInfo, cfg.Info, l.colorize)
	l.levels[LevelWarn] = newLevelState(LevelWarn, cfg.Warn, l.colorize)
	l.levels[LevelError] = newLevelState(LevelError, cfg.Error, l.colorize)

	return l
}

// Close releases any resources held by the Logger (e.g. file handles).
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	var firstErr error
	for _, c := range l.closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Init replaces the package logger with one built from cfg.
func Init(cfg Config) {
	std = New(cfg)
}

// Std returns the current package-level logger.
func Std() *Logger {
	return std
}

// Info logs a message at info level.
func Info(args ...any) {
	Std().log(LevelInfo, func() string {
		return fmt.Sprint(args...)
	})
}

// Infof logs a formatted message at info level.
func Infof(format string, args ...any) {
	Std().log(LevelInfo, func() string {
		return fmt.Sprintf(format, args...)
	})
}

// Debug logs a message at debug level.
func Debug(args ...any) {
	Std().log(LevelDebug, func() string {
		return fmt.Sprint(args...)
	})
}

// Debugf logs a formatted message at debug level.
func Debugf(format string, args ...any) {
	Std().log(LevelDebug, func() string {
		return fmt.Sprintf(format, args...)
	})
}

// Warn logs a message at warn level.
func Warn(args ...any) {
	Std().log(LevelWarn, func() string {
		return fmt.Sprint(args...)
	})
}

// Warnf logs a formatted message at warn level.
func Warnf(format string, args ...any) {
	Std().log(LevelWarn, func() string {
		return fmt.Sprintf(format, args...)
	})
}

// Error logs a message at error level.
func Error(args ...any) {
	Std().log(LevelError, func() string {
		return fmt.Sprint(args...)
	})
}

// Errorf logs a formatted message at error level.
func Errorf(format string, args ...any) {
	Std().log(LevelError, func() string {
		return fmt.Sprintf(format, args...)
	})
}

// SetEnabled toggles a logging level on or off.
func (l *Logger) SetEnabled(level Level, enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if st, ok := l.levels[level]; ok {
		st.enabled = enabled
	}
}

// Enabled reports whether the supplied level is currently active.
func (l *Logger) Enabled(level Level) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if st, ok := l.levels[level]; ok {
		return st.enabled
	}
	return false
}

// SetFlags updates the log.Flags used for the supplied level.
func (l *Logger) SetFlags(level Level, flags int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if st, ok := l.levels[level]; ok {
		st.logger.SetFlags(flags)
	}
}

// SetLevelOutput routes a level to the provided writer.
func (l *Logger) SetLevelOutput(level Level, w io.Writer) {
	if w == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if st, ok := l.levels[level]; ok {
		st.logger.SetOutput(w)
	}
}

// SetOutput routes all log levels to the provided writer.
func (l *Logger) SetOutput(w io.Writer) {
	if w == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	for _, st := range l.levels {
		st.logger.SetOutput(w)
	}
}

// SetPrefix overrides the textual prefix for a given level.
func (l *Logger) SetPrefix(level Level, prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if st, ok := l.levels[level]; ok {
		if prefix == "" {
			prefix = fmt.Sprintf("[%s]", level.String())
		}
		st.basePrefix = prefix
		st.updatePrefix(l.colorize)
	}
}

// EnableColors turns on ANSI colorized prefixes.
func (l *Logger) EnableColors() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.colorize {
		return
	}

	l.colorize = true
	for _, st := range l.levels {
		st.updatePrefix(true)
	}
}

// DisableColors turns off ANSI colorized prefixes.
func (l *Logger) DisableColors() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.colorize {
		return
	}

	l.colorize = false
	for _, st := range l.levels {
		st.updatePrefix(false)
	}
}

// ColorsEnabled reports whether ANSI colors are currently active.
func (l *Logger) ColorsEnabled() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.colorize
}

func (l *Logger) log(level Level, build func() string) {
	if build == nil {
		return
	}

	l.mu.RLock()
	st, ok := l.levels[level]
	enabled := ok && st.enabled
	l.mu.RUnlock()

	if !enabled {
		return
	}

	st.output(build())
}

func (cfg Config) withDefaults() Config {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}

	cfg.Debug = cfg.Debug.withDefaults(LevelDebug, false, cfg.Output)
	cfg.Info = cfg.Info.withDefaults(LevelInfo, true, cfg.Output)
	cfg.Warn = cfg.Warn.withDefaults(LevelWarn, true, cfg.Output)
	cfg.Error = cfg.Error.withDefaults(LevelError, true, cfg.Output)

	return cfg
}

func (opts LevelOptions) withDefaults(level Level, defaultEnabled bool, defaultWriter io.Writer) LevelOptions {
	if opts.Enabled == nil {
		opts.Enabled = boolPtr(defaultEnabled)
	}
	if opts.Flags == nil {
		opts.Flags = intPtr(log.LstdFlags)
	}
	if opts.Prefix == "" {
		opts.Prefix = fmt.Sprintf("[%s]", level.String())
	}
	if opts.Writer == nil {
		opts.Writer = defaultWriter
	}

	return opts
}

func newLevelState(level Level, opts LevelOptions, colorize bool) *levelState {
	st := &levelState{
		logger:     log.New(opts.Writer, "", *opts.Flags),
		enabled:    *opts.Enabled,
		basePrefix: opts.Prefix,
		color:      levelColors[level],
	}
	st.updatePrefix(colorize)
	return st
}

func (st *levelState) updatePrefix(colorize bool) {
	prefix := st.basePrefix
	if colorize && st.color != "" {
		prefix = st.color + prefix + ansiReset
	}
	st.logger.SetPrefix(prefix + " ")
}

func (st *levelState) output(msg string) {
	if msg == "" {
		msg = "(empty message)"
	}
	_ = st.logger.Output(defaultCallDepth, msg)
}

func boolPtr(v bool) *bool {
	return &v
}

func intPtr(v int) *int {
	return &v
}
