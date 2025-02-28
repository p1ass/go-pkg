# sloggcloud

sloggcloud は、Go の標準ログライブラリ [slog](https://pkg.go.dev/log/slog) 用の Google Cloud Logging ハンドラーを提供するパッケージです。

## 特徴

- Google Cloud Logging と互換性のある構造化ログ出力
- OpenTelemetry のトレース情報（トレースIDとスパンID）の自動付与
- ソースコードの位置情報の出力サポート
- ログレベルのフィルタリング
- 属性（フィールド）の柔軟な追加


## 使い方

### 基本的な使用方法

```go
package main

import (
    "log/slog"
    "os"

    "github.com/p1ass/go-pkg/sloggcloud"
)

func main() {
    // ハンドラーの作成
    handler := sloggcloud.New(os.Stdout)
    
    // ロガーの作成
    logger := slog.New(handler)
    
    // ログの出力
    logger.Info("hello", "user", "alice")
}
```

### オプションの設定

```go
handler := sloggcloud.New(os.Stdout,
    // ログレベルの設定
    sloggcloud.WithLevel(slog.LevelDebug),
    // ソースコードの位置情報を出力
    sloggcloud.WithSource(true),
    // Google Cloud Project ID の設定
    sloggcloud.WithProjectID("your-project-id"),
)
```

### OpenTelemetry とのインテグレーション

```go
ctx := context.Background()
tracer := otel.Tracer("example")

ctx, span := tracer.Start(ctx, "operation")
defer span.End()

// トレース情報が自動的にログに含まれます
logger.InfoContext(ctx, "operation started")
```

## オプション

| オプション | 説明 | デフォルト値 |
|------------|------|--------------|
| `WithLevel` | 最小ログレベルを設定 | `slog.LevelInfo` |
| `WithSource` | ソースコードの位置情報の出力を有効化 | `true` |
| `WithProjectID` | Google Cloud Project ID を設定 | `""` |

## 出力形式

```json
{
  "severity": "INFO",
  "time": "2024-01-01T12:00:00.000Z",
  "msg": "hello",
  "logging.googleapis.com/trace": "projects/your-project-id/traces/trace-id",
  "logging.googleapis.com/spanId": "span-id",
  "logging.googleapis.com/sourceLocation": {
    "file": "main.go",
    "line": 15,
    "function": "main.main"
  }
}
```
