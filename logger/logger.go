package logger

import (
    "context"
    "fmt"
    "io"
    "log/slog"
    "os"
    "sync"
)

type CustomHandler struct {
    mu     sync.Mutex
    w      io.Writer
    level  *slog.LevelVar
    attrs  []slog.Attr
    groups []string
}

const (
    colorReset  = "\033[0m"
    colorRed    = "\033[31m"
    colorGreen  = "\033[32m"
    colorYellow = "\033[33m"
    colorBlue   = "\033[34m"
)

func (h *CustomHandler) Handle(ctx context.Context, r slog.Record) error {
    level := r.Level.String()
    timeStr := r.Time.Format("15:04:05.000")

    var levelColor string
    switch r.Level {
    case slog.LevelDebug:
        levelColor = colorBlue
    case slog.LevelInfo:
        levelColor = colorGreen
    case slog.LevelWarn:
        levelColor = colorYellow
    case slog.LevelError:
        levelColor = colorRed
    default:
        levelColor = colorReset
    }

    coloredLevel := fmt.Sprintf("%s%s%s", levelColor, level, colorReset)

    msg := r.Message

    h.mu.Lock()
    defer h.mu.Unlock()

    output := fmt.Sprintf("%s %s %s", timeStr, coloredLevel, msg)

    // Handle additional attributes
    r.Attrs(func(a slog.Attr) bool {
        output += fmt.Sprintf(" %s=%v", a.Key, a.Value.Any())
        return true
    })

    _, err := fmt.Fprintln(h.w, output)
    return err
}

func (h *CustomHandler) Enabled(ctx context.Context, level slog.Level) bool {
    return level >= h.level.Level()
}

func (h *CustomHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    h2 := h.clone()
    h2.attrs = append(h2.attrs, attrs...)
    return h2
}

func (h *CustomHandler) WithGroup(name string) slog.Handler {
    h2 := h.clone()
    h2.groups = append(h2.groups, name)
    return h2
}

func (h *CustomHandler) clone() *CustomHandler {
    return &CustomHandler{
        w:      h.w,
        level:  h.level,
        attrs:  append([]slog.Attr{}, h.attrs...),
        groups: append([]string{}, h.groups...),
    }
}

func NewCustomHandler(w io.Writer, opts *slog.HandlerOptions) *CustomHandler {
    level := new(slog.LevelVar)
    level.Set(slog.LevelInfo) // Default to Info level
    h := &CustomHandler{w: w, level: level}
    if opts != nil {
        if opts.Level != nil {
            level.Set(opts.Level.Level())
        }
        if opts.AddSource {
        	//
        }
    }
    return h
}

var Slog *slog.Logger

func init() {
    Slog = slog.New(NewCustomHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    slog.SetDefault(Slog)
}

func SetLogLevel(val string) {
    var level slog.Level
    err := level.UnmarshalText([]byte(val))
    if err != nil {
        Slog.Info("could not parse loglevel, keeping as is")
        return
    }

    handler, ok := Slog.Handler().(*CustomHandler)
    if !ok {
        Slog.Info("Handler is not a CustomHandler, cannot change log level")
        return
    }

    handler.level.Set(level)
    Slog.Info("Log level changed", "level", level)
}
