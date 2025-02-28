package sloggcloud

import (
	"log/slog"
)

// options はハンドラーの設定オプションを保持する構造体です。
type options struct {
	level     slog.Level
	addSource bool
	projectID string
}

// Option はハンドラーを設定するための関数型です。
type Option func(*options)

// defaultOptions はデフォルトのオプション値を返します。
func defaultOptions() *options {
	return &options{
		level:     slog.LevelInfo,
		addSource: false,
		projectID: "",
	}
}

// WithLevel は最小ログレベルを設定します。
func WithLevel(level slog.Level) Option {
	return func(o *options) {
		o.level = level
	}
}

// WithSource はソースコードの位置情報の出力を有効にします。
func WithSource(enabled bool) Option {
	return func(o *options) {
		o.addSource = enabled
	}
}

// WithProjectID は Google Cloud Project ID を設定します。
func WithProjectID(projectID string) Option {
	return func(o *options) {
		o.projectID = projectID
	}
}
