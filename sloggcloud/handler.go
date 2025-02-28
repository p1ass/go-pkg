// Package sloggcloud は Google Cloud Logging 用の slog.Handler 実装を提供します。
package sloggcloud

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// Handler は Google Cloud Logging 用の slog.Handler 実装です。
// Google Cloud Logging と互換性のある構造化フォーマットでログを出力します。
// また、利用可能な場合は OpenTelemetry のトレース ID とスパン ID も含みます。
type Handler struct {
	opts    *options
	attrs   []slog.Attr
	groups  []string
	w       io.Writer
	program string
}

// New は Google Cloud Logging 用の新しい Handler を作成します。
func New(w io.Writer, opts ...Option) *Handler {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &Handler{
		opts:    o,
		w:       w,
		program: o.program,
	}
}

// Enabled は指定されたレベルのレコードをハンドラが処理するかどうかを報告します。
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.level
}

// Handle はレコードを処理します。
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	// 新しいslogロガーを作成
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

	// 属性を格納するスライスを作成
	attrs := make([]slog.Attr, 0)

	// ソース位置情報が有効な場合は追加
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

	// OpenTelemetry コンテキストからトレース ID とスパン ID を抽出
	if h.opts.addTraceInfo {
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
	}

	// 事前定義された属性を追加
	if len(h.attrs) > 0 || len(h.groups) > 0 {
		attrMap := make(map[string]interface{})
		for _, attr := range h.attrs {
			addAttr(attrMap, h.groups, attr)
		}
		attrs = append(attrs, slog.Any("attributes", attrMap))
	}

	// レコードの属性を追加
	recordAttrs := make(map[string]interface{})
	r.Attrs(func(attr slog.Attr) bool {
		addAttr(recordAttrs, h.groups, attr)
		return true
	})
	if len(recordAttrs) > 0 {
		if len(h.attrs) == 0 && len(h.groups) == 0 {
			attrs = append(attrs, slog.Any("attributes", recordAttrs))
		} else {
			// 既存の attributes に追加
			lastAttr := &attrs[len(attrs)-1]
			if lastAttr.Key == "attributes" {
				if existingAttrs, ok := lastAttr.Value.Any().(map[string]interface{}); ok {
					for k, v := range recordAttrs {
						existingAttrs[k] = v
					}
				}
			}
		}
	}

	// ログを出力
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

// levelToSeverity は slog.Level を Google Cloud Logging の重要度に変換します。
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

// addAttr は属性を属性マップに追加します。
func addAttr(attrs map[string]interface{}, groups []string, attr slog.Attr) {
	if attr.Equal(slog.Attr{}) {
		return
	}

	// グループを処理
	if len(groups) > 0 {
		// グループ用のネストされたマップを作成
		current := attrs
		for _, g := range groups[:len(groups)-1] {
			v, ok := current[g]
			if !ok {
				v = make(map[string]interface{})
				current[g] = v
			}

			next, ok := v.(map[string]interface{})
			if !ok {
				// 値がマップでない場合は新しいマップを作成
				next = make(map[string]interface{})
				current[g] = next
			}
			current = next
		}

		// 最も内側のグループに属性を追加
		lastGroup := groups[len(groups)-1]
		v, ok := current[lastGroup]
		if !ok {
			v = make(map[string]interface{})
			current[lastGroup] = v
		}

		innerMap, ok := v.(map[string]interface{})
		if !ok {
			innerMap = make(map[string]interface{})
			current[lastGroup] = innerMap
		}

		innerMap[attr.Key] = attrValue(attr.Value)
		return
	}

	// グループがない場合は直接属性マップに追加
	attrs[attr.Key] = attrValue(attr.Value)
}

// attrValue は slog.Value を JSON マーシャリングに適した Go の値に変換します。
func attrValue(v slog.Value) interface{} {
	switch v.Kind() {
	case slog.KindBool:
		return v.Bool()
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindFloat64:
		return v.Float64()
	case slog.KindInt64:
		return v.Int64()
	case slog.KindString:
		return v.String()
	case slog.KindTime:
		return v.Time().Format(time.RFC3339Nano)
	case slog.KindUint64:
		return v.Uint64()
	case slog.KindGroup:
		attrs := v.Group()
		m := make(map[string]interface{}, len(attrs))
		for _, attr := range attrs {
			m[attr.Key] = attrValue(attr.Value)
		}
		return m
	case slog.KindLogValuer:
		return attrValue(v.LogValuer().LogValue())
	default:
		return fmt.Sprintf("%v", v)
	}
}
