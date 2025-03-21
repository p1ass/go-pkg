// Package sloggcloud は Google Cloud Logging 用の slog.Handler 実装を提供します。
package sloggcloud

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"

	"go.opentelemetry.io/otel/trace"
)

// Handler は Google Cloud Logging 用の slog.Handler 実装です。
// Google Cloud Logging と互換性のある構造化フォーマットでログを出力します。
// また、利用可能な場合は OpenTelemetry のトレース ID とスパン ID も含みます。
type Handler struct {
	opts   *options
	attrs  []slog.Attr
	groups []string
	w      io.Writer
}

var _ slog.Handler = (*Handler)(nil)

// New は Google Cloud Logging 用の新しい Handler を作成します。
func New(w io.Writer, opts ...Option) *Handler {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &Handler{
		opts: o,
		w:    w,
	}
}

// Enabled は指定されたレベルのレコードをハンドラが処理するかどうかを報告します。
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.level
}

// Handle はレコードを処理します。
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	logger := slog.New(slog.NewJSONHandler(h.w, &slog.HandlerOptions{
		Level: h.opts.level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// levelをseverityに変換
			if a.Key == slog.LevelKey {
				return slog.String("severity", levelToSeverity(r.Level))
			}
			return a
		},
	}))

	attrs := make([]slog.Attr, 0)

	if h.opts.addSource {
		var frame runtime.Frame
		pc := r.PC
		if pc != 0 {
			frames := runtime.CallersFrames([]uintptr{pc})
			frame, _ = frames.Next()
			attrs = append(attrs,
				slog.Group("logging.googleapis.com/sourceLocation",
					slog.String("file", frame.File),
					slog.Int("line", frame.Line),
					slog.String("function", frame.Function),
				),
			)
		}
	}

	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		traceID := span.SpanContext().TraceID()
		spanID := span.SpanContext().SpanID()

		// Google Cloud Logging の要件に従ってトレース ID をフォーマット
		var traceIDStr string
		if h.opts.projectID != "" {
			traceIDStr = fmt.Sprintf("projects/%s/traces/%s", h.opts.projectID, traceID.String())
		} else {
			traceIDStr = traceID.String()
		}

		attrs = append(attrs,
			slog.String("logging.googleapis.com/trace", traceIDStr),
			slog.String("logging.googleapis.com/spanId", spanID.String()),
		)
	}

	if len(h.attrs) > 0 {
		attrs = append(attrs, h.attrs...)
	}

	r.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})

	if len(h.groups) > 0 {
		var values []any
		for _, attr := range attrs {
			values = append(values, attr)
		}
		groupedAttrs := values
		for i := len(h.groups) - 1; i >= 0; i-- {
			groupedAttrs = []any{slog.Group(h.groups[i], groupedAttrs...)}
		}
		attrs = []slog.Attr{groupedAttrs[0].(slog.Attr)}
	}

	logger.LogAttrs(ctx, r.Level, r.Message, attrs...)
	return nil
}

// WithAttrs は指定された属性を持つ新しい Handler を返します。
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	h2 := *h
	h2.attrs = append(h2.attrs, attrs...)
	return &h2
}

// WithGroup は指定されたグループを持つ新しい Handler を返します。
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	h2 := *h
	h2.groups = append(h2.groups, name)
	return &h2
}

func levelToSeverity(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "ERROR"
	case level >= slog.LevelWarn && level < slog.LevelError:
		return "WARNING"
	case level >= slog.LevelInfo && level < slog.LevelWarn:
		return "INFO"
	default:
		return "DEBUG"
	}
}
