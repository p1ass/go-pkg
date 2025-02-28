package sloggcloud_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/p1ass/go-pkg/sloggcloud"
	"go.opentelemetry.io/otel/trace"
)

// TestHandler_Handle は Handler の Handle メソッドをテストします。
func TestHandler_Handle(t *testing.T) {
	tests := []struct {
		name    string
		level   slog.Level
		message string
		attrs   []slog.Attr
		opts    []sloggcloud.Option
		want    map[string]interface{}
	}{
		{
			name:    "基本的なINFOレベルのログ",
			level:   slog.LevelInfo,
			message: "test message",
			attrs:   []slog.Attr{slog.String("key", "value")},
			opts:    []sloggcloud.Option{},
			want: map[string]interface{}{
				"severity": "INFO",
				"msg":      "test message",
				"key":      "value",
			},
		},
		{
			name:    "ソース情報付きのエラーログ",
			level:   slog.LevelError,
			message: "error message",
			attrs:   []slog.Attr{slog.Int("code", 500)},
			opts:    []sloggcloud.Option{sloggcloud.WithSource(true)},
			want: map[string]interface{}{
				"severity": "ERROR",
				"msg":      "error message",
				"code":     float64(500),
				"logging.googleapis.com/sourceLocation": map[string]interface{}{
					"file":     "", // 実際のファイル名はランタイムによって決まるため、存在チェックのみ
					"line":     0,  // 同上
					"function": "", // 同上
				},
			},
		},
		{
			name:    "プロジェクトID付きの警告ログ",
			level:   slog.LevelWarn,
			message: "warning with project ID",
			attrs:   []slog.Attr{},
			opts:    []sloggcloud.Option{sloggcloud.WithProjectID("test-project")},
			want: map[string]interface{}{
				"severity": "WARNING",
				"msg":      "warning with project ID",
			},
		},
		{
			name:    "グループ化された属性を持つログ",
			level:   slog.LevelInfo,
			message: "message with group",
			attrs:   []slog.Attr{slog.String("in_group", "value")},
			opts:    []sloggcloud.Option{},
			want: map[string]interface{}{
				"severity": "INFO",
				"msg":      "message with group",
				"in_group": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// テスト用のバッファを準備
			var buf bytes.Buffer
			handler := sloggcloud.New(&buf, tt.opts...)

			// コンテキストの準備
			ctx := context.Background()
			if tt.name == "プロジェクトID付きの警告ログ" {
				traceID, _ := trace.TraceIDFromHex("01020304050607080102030405060708")
				spanID, _ := trace.SpanIDFromHex("0102030405060708")
				spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
					TraceID:    traceID,
					SpanID:     spanID,
					TraceFlags: trace.FlagsSampled,
				})
				ctx = trace.ContextWithSpanContext(ctx, spanCtx)
			}

			// ログの出力
			if tt.name == "グループ化された属性を持つログ" {
				groupHandler := handler.WithGroup("group")
				groupHandler = groupHandler.WithAttrs([]slog.Attr{slog.String("in_group", "value")})
				logger := slog.New(groupHandler)
				logger.LogAttrs(ctx, tt.level, tt.message)
			} else {
				logger := slog.New(handler)
				if len(tt.attrs) > 0 {
					attrHandler := logger.Handler().WithAttrs(tt.attrs)
					logger = slog.New(attrHandler)
				}
				logger.Log(ctx, tt.level, tt.message)
			}

			// 出力結果のパース
			var got map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("JSONのパースに失敗: %v", err)
			}

			// time フィールドの検証
			timeStr, ok := got["time"].(string)
			if !ok {
				t.Errorf("time フィールドが文字列ではありません: %T", got["time"])
			} else if _, err := time.Parse(time.RFC3339Nano, timeStr); err != nil {
				t.Errorf("time フィールドのパースに失敗: %v", err)
			}

			// 期待されるフィールドの検証
			for k, v := range tt.want {
				gotVal, ok := got[k]
				if !ok {
					t.Errorf("フィールド %s が見つかりません", k)
					continue
				}

				switch k {
				case "logging.googleapis.com/sourceLocation":
					sourceLocation, ok := gotVal.(map[string]interface{})
					if !ok {
						t.Errorf("sourceLocation がマップではありません: %T", gotVal)
					} else {
						for field := range sourceLocation {
							if field != "file" && field != "line" && field != "function" {
								t.Errorf("予期しないフィールド: %s", field)
							}
						}
					}
				case "attributes":
					expectedAttrs := v.(map[string]interface{})
					gotAttrs, ok := gotVal.(map[string]interface{})
					if !ok {
						t.Errorf("attributes がマップではありません: %T", gotVal)
						continue
					}
					if diff := cmp.Diff(expectedAttrs, gotAttrs); diff != "" {
						t.Errorf("属性が異なります (-want +got):\n%s", diff)
					}
				default:
					if diff := cmp.Diff(v, gotVal); diff != "" {
						t.Errorf("フィールド %s の値が異なります (-want +got):\n%s", k, diff)
					}
				}
			}

			// トレース情報の検証
			if tt.name == "プロジェクトID付きの警告ログ" {
				traceID, ok := got["logging.googleapis.com/trace"].(string)
				if !ok {
					t.Errorf("trace ID が文字列ではありません: %T", got["logging.googleapis.com/trace"])
				} else if !strings.HasPrefix(traceID, "projects/test-project/traces/") {
					t.Errorf("trace ID が不正です: %s", traceID)
				}

				spanID, ok := got["logging.googleapis.com/spanId"].(string)
				if !ok {
					t.Errorf("span ID が文字列ではありません: %T", got["logging.googleapis.com/spanId"])
				} else if spanID != "0102030405060708" {
					t.Errorf("span ID が不正です: %s", spanID)
				}
			}
		})
	}
}
