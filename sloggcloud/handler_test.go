package sloggcloud_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/p1ass/go-pkg/sloggcloud"
	"go.opentelemetry.io/otel/trace"
)

func TestHandler_Handle(t *testing.T) {
	tests := []struct {
		name              string
		level             slog.Level
		message           string
		args              []any
		opts              []sloggcloud.Option
		setupTrace        func() context.Context
		want              map[string]interface{}
		hasSourceLocation bool
	}{
		{
			name:    "基本的なログ出力",
			level:   slog.LevelInfo,
			message: "test message",
			args:    []any{"key", "value"},
			opts:    []sloggcloud.Option{},
			setupTrace: func() context.Context {
				return context.Background()
			},
			want: map[string]interface{}{
				"severity": "INFO",
				"msg":      "test message",
				"key":      "value",
			},
			hasSourceLocation: false,
		},
		{
			name:    "ソース情報付きのログ",
			level:   slog.LevelInfo,
			message: "message with source",
			args:    []any{"code", 500},
			opts:    []sloggcloud.Option{sloggcloud.WithSource(true)},
			setupTrace: func() context.Context {
				return context.Background()
			},
			want: map[string]interface{}{
				"severity": "INFO",
				"msg":      "message with source",
				"code":     float64(500),
			},
			hasSourceLocation: true,
		},
		{
			name:    "プロジェクトIDとトレース情報付きのログ",
			level:   slog.LevelInfo,
			message: "message with project ID",
			args:    []any{},
			opts:    []sloggcloud.Option{sloggcloud.WithProjectID("test-project")},
			setupTrace: func() context.Context {
				traceID, _ := trace.TraceIDFromHex("01020304050607080102030405060708")
				spanID, _ := trace.SpanIDFromHex("0102030405060708")
				spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
					TraceID:    traceID,
					SpanID:     spanID,
					TraceFlags: trace.FlagsSampled,
				})
				return trace.ContextWithSpanContext(context.Background(), spanCtx)
			},
			want: map[string]interface{}{
				"severity":                      "INFO",
				"msg":                           "message with project ID",
				"logging.googleapis.com/trace":  "projects/test-project/traces/01020304050607080102030405060708",
				"logging.googleapis.com/spanId": "0102030405060708",
			},
			hasSourceLocation: false,
		},
		// レベルのテストケース
		{
			name:    "DEBUGレベルのログ",
			level:   slog.LevelDebug,
			message: "debug message",
			args:    []any{"key", "value"},
			opts: []sloggcloud.Option{
				sloggcloud.WithLevel(slog.LevelDebug),
			},
			setupTrace: func() context.Context {
				return context.Background()
			},
			want: map[string]interface{}{
				"severity": "DEBUG",
				"msg":      "debug message",
				"key":      "value",
			},
			hasSourceLocation: false,
		},
		{
			name:    "WARNレベルのログ",
			level:   slog.LevelWarn,
			message: "warning message",
			args:    []any{"key", "value"},
			opts:    []sloggcloud.Option{},
			setupTrace: func() context.Context {
				return context.Background()
			},
			want: map[string]interface{}{
				"severity": "WARNING",
				"msg":      "warning message",
				"key":      "value",
			},
			hasSourceLocation: false,
		},
		{
			name:    "ERRORレベルのログ",
			level:   slog.LevelError,
			message: "error message",
			args:    []any{"key", "value"},
			opts:    []sloggcloud.Option{},
			setupTrace: func() context.Context {
				return context.Background()
			},
			want: map[string]interface{}{
				"severity": "ERROR",
				"msg":      "error message",
				"key":      "value",
			},
			hasSourceLocation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := sloggcloud.New(&buf, tt.opts...)
			logger := slog.New(handler)

			ctx := tt.setupTrace()
			logger.Log(ctx, tt.level, tt.message, tt.args...)

			var got map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("JSONのパースに失敗: %v", err)
			}

			// time フィールドの検証
			if _, ok := got["time"].(string); !ok {
				t.Error("time フィールドが文字列ではありません")
			}
			delete(got, "time")

			// ソース位置情報の検証
			if sourceLocation, ok := got["logging.googleapis.com/sourceLocation"].(map[string]interface{}); ok {
				if !tt.hasSourceLocation {
					t.Error("予期しないソース位置情報が含まれています")
				}
				if _, ok := sourceLocation["file"].(string); !ok {
					t.Error("sourceLocation.file が文字列ではありません")
				}
				if _, ok := sourceLocation["line"].(float64); !ok {
					t.Error("sourceLocation.line が数値ではありません")
				}
				if _, ok := sourceLocation["function"].(string); !ok {
					t.Error("sourceLocation.function が文字列ではありません")
				}
				delete(got, "logging.googleapis.com/sourceLocation")
			} else if tt.hasSourceLocation {
				t.Error("ソース位置情報が含まれていません")
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("出力が異なります (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHandler_WithAttrs(t *testing.T) {
	tests := []struct {
		name              string
		level             slog.Level
		message           string
		attrs             []slog.Attr
		opts              []sloggcloud.Option
		setupTrace        func() context.Context
		want              map[string]interface{}
		hasSourceLocation bool
	}{
		{
			name:    "WithAttrsで属性を追加",
			level:   slog.LevelInfo,
			message: "message with attrs",
			attrs: []slog.Attr{
				slog.String("service", "test-service"),
			},
			opts: []sloggcloud.Option{},
			setupTrace: func() context.Context {
				return context.Background()
			},
			want: map[string]interface{}{
				"severity": "INFO",
				"msg":      "message with attrs",
				"service":  "test-service",
			},
			hasSourceLocation: false,
		},
		{
			name:    "WithAttrsで複数回属性を追加",
			level:   slog.LevelInfo,
			message: "message with multiple attrs",
			attrs: []slog.Attr{
				slog.String("service", "test-service"),
				slog.Int("version", 1),
				slog.String("environment", "test"),
			},
			opts: []sloggcloud.Option{},
			setupTrace: func() context.Context {
				return context.Background()
			},
			want: map[string]interface{}{
				"severity":    "INFO",
				"msg":         "message with multiple attrs",
				"service":     "test-service",
				"version":     float64(1),
				"environment": "test",
			},
			hasSourceLocation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := sloggcloud.New(&buf, tt.opts...)
			logger := slog.New(handler.WithAttrs(tt.attrs))

			ctx := tt.setupTrace()
			logger.Log(ctx, tt.level, tt.message)

			var got map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("JSONのパースに失敗: %v", err)
			}

			// time フィールドの検証
			if _, ok := got["time"].(string); !ok {
				t.Error("time フィールドが文字列ではありません")
			}
			delete(got, "time")

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("出力が異なります (-want +got):\n%s", diff)
			}
		})
	}
}
