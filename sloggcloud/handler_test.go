package sloggcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// TestHandler_Handle tests the Handle method of the Handler.
func TestHandler_Handle(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name           string
		level          slog.Level
		message        string
		attrs          []slog.Attr
		opts           []Option
		expectedFields map[string]interface{}
	}{
		{
			name:    "basic info log",
			level:   slog.LevelInfo,
			message: "test message",
			attrs:   []slog.Attr{slog.String("key", "value")},
			opts:    []Option{},
			expectedFields: map[string]interface{}{
				"severity": "INFO",
				"msg":      "test message",
				"attributes": map[string]interface{}{
					"key": "value",
				},
			},
		},
		{
			name:    "error log with source",
			level:   slog.LevelError,
			message: "error message",
			attrs:   []slog.Attr{slog.Int("code", 500)},
			opts:    []Option{WithSource(true)},
			expectedFields: map[string]interface{}{
				"severity": "ERROR",
				"msg":      "error message",
				"attributes": map[string]interface{}{
					"code": float64(500),
				},
				"logging.googleapis.com/sourceLocation": map[string]interface{}{
					"file":     "", // 実際のファイル名はランタイムによって決まるため、存在チェックのみ
					"line":     0,  // 同上
					"function": "", // 同上
				},
			},
		},
		{
			name:    "log with project ID",
			level:   slog.LevelWarn,
			message: "warning with project ID",
			attrs:   []slog.Attr{},
			opts:    []Option{WithProjectID("test-project")},
			expectedFields: map[string]interface{}{
				"severity": "WARNING",
				"msg":      "warning with project ID",
			},
		},
		{
			name:    "log with group",
			level:   slog.LevelInfo,
			message: "message with group",
			attrs:   []slog.Attr{},
			opts:    []Option{},
			expectedFields: map[string]interface{}{
				"severity": "INFO",
				"msg":      "message with group",
				"attributes": map[string]interface{}{
					"group": map[string]interface{}{
						"in_group": "value",
					},
				},
			},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			var buf bytes.Buffer
			handler := New(&buf, tc.opts...)

			// Create a context with a span for testing trace extraction
			ctx := context.Background()
			if tc.name == "log with project ID" {
				// Create a mock span context with valid trace and span IDs
				traceID, _ := trace.TraceIDFromHex("01020304050607080102030405060708")
				spanID, _ := trace.SpanIDFromHex("0102030405060708")

				// Create a span context with the mock trace and span IDs
				spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
					TraceID:    traceID,
					SpanID:     spanID,
					TraceFlags: trace.FlagsSampled,
				})

				// Set the span context in the context
				ctx = trace.ContextWithSpanContext(ctx, spanCtx)
			}

			// If the test case is for groups, add a group
			if tc.name == "log with group" {
				groupHandler := handler.WithGroup("group")
				groupHandler = groupHandler.WithAttrs([]slog.Attr{slog.String("in_group", "value")})
				logger := slog.New(groupHandler)

				// Log the message
				logger.LogAttrs(ctx, tc.level, tc.message)
			} else {
				// Create a new logger with the handler
				logger := slog.New(handler)

				// Add attributes if any
				if len(tc.attrs) > 0 {
					attrHandler := logger.Handler().WithAttrs(tc.attrs)
					logger = slog.New(attrHandler)
				}

				// Log the message
				logger.Log(ctx, tc.level, tc.message)
			}

			// Parse the JSON output
			var got map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			// Check expected fields
			for k, v := range tc.expectedFields {
				gotVal, ok := got[k]
				if !ok {
					t.Errorf("Expected field %s not found in output", k)
					continue
				}

				switch k {
				case "logging.googleapis.com/sourceLocation":
					// ソースロケーションの場合は、フィールドの存在のみを確認
					sourceLocation, ok := gotVal.(map[string]interface{})
					if !ok {
						t.Errorf("Expected sourceLocation to be a map, got %T", gotVal)
					} else {
						for field := range sourceLocation {
							if field != "file" && field != "line" && field != "function" {
								t.Errorf("Unexpected field in sourceLocation: %s", field)
							}
						}
					}
				case "attributes":
					// 属性の場合は、期待される属性が含まれているか確認
					expectedAttrs := v.(map[string]interface{})
					gotAttrs, ok := gotVal.(map[string]interface{})
					if !ok {
						t.Errorf("Expected attributes to be a map, got %T", gotVal)
						continue
					}
					for attrKey, attrVal := range expectedAttrs {
						if !reflect.DeepEqual(gotAttrs[attrKey], attrVal) {
							t.Errorf("Attribute %s: expected %v, got %v", attrKey, attrVal, gotAttrs[attrKey])
						}
					}
				default:
					if !reflect.DeepEqual(gotVal, v) {
						t.Errorf("Field %s: expected %v, got %v", k, v, gotVal)
					}
				}
			}

			// Check time field exists and is parseable
			timeStr, ok := got["time"].(string)
			if !ok {
				t.Errorf("Expected time to be a string, got %T", got["time"])
			} else {
				_, err := time.Parse(time.RFC3339Nano, timeStr)
				if err != nil {
					t.Errorf("Failed to parse time: %v", err)
				}
			}

			// Check trace fields if project ID is set
			if tc.name == "log with project ID" {
				traceID, ok := got["logging.googleapis.com/trace"].(string)
				if !ok {
					t.Errorf("Expected trace ID to be a string, got %T", got["logging.googleapis.com/trace"])
				}
				if !strings.HasPrefix(traceID, "projects/test-project/traces/") {
					t.Errorf("Expected trace ID to start with projects/test-project/traces/, got %s", traceID)
				}

				spanID, ok := got["logging.googleapis.com/spanId"].(string)
				if !ok {
					t.Errorf("Expected span ID to be a string, got %T", got["logging.googleapis.com/spanId"])
				}
				if spanID != "0102030405060708" {
					t.Errorf("Expected span ID to be 0102030405060708, got %s", spanID)
				}
			}
		})
	}
}

// TestHandler_Enabled tests the Enabled method of the Handler.
func TestHandler_Enabled(t *testing.T) {
	testCases := []struct {
		name     string
		level    slog.Level
		minLevel slog.Level
		want     bool
	}{
		{
			name:     "info enabled for info level",
			level:    slog.LevelInfo,
			minLevel: slog.LevelInfo,
			want:     true,
		},
		{
			name:     "debug disabled for info level",
			level:    slog.LevelDebug,
			minLevel: slog.LevelInfo,
			want:     false,
		},
		{
			name:     "error enabled for info level",
			level:    slog.LevelError,
			minLevel: slog.LevelInfo,
			want:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := New(&bytes.Buffer{}, WithLevel(tc.minLevel))
			got := handler.Enabled(context.Background(), tc.level)
			if got != tc.want {
				t.Errorf("Enabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestLevelToSeverity tests the levelToSeverity function.
func TestLevelToSeverity(t *testing.T) {
	testCases := []struct {
		name  string
		level slog.Level
		want  string
	}{
		{
			name:  "debug level",
			level: slog.LevelDebug,
			want:  "DEBUG",
		},
		{
			name:  "info level",
			level: slog.LevelInfo,
			want:  "INFO",
		},
		{
			name:  "warn level",
			level: slog.LevelWarn,
			want:  "WARNING",
		},
		{
			name:  "error level",
			level: slog.LevelError,
			want:  "ERROR",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := levelToSeverity(tc.level)
			if got != tc.want {
				t.Errorf("levelToSeverity() = %v, want %v", got, tc.want)
			}
		})
	}
}
