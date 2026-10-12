# MCPレスポンスフォーマット

`mymcpserve`の全ツールは、成功・エラーにかかわらず、同じ`structuredContent`エンベロープを返します。これにより、AI Agentや`mymcp`のようなCLIは、結果本体とエラーを同じJSON処理経路で扱えます。

## 成功時

MCPのレスポンスは次の形です。`content`は重複出力を避けるため空配列です。

```json
{
  "content": [],
  "structuredContent": {
    "data": [{"id": "item-1"}],
    "meta": {
      "schema_version": "1",
      "tool": "list_items",
      "requested_at": "2026-10-12T09:00:00Z",
      "data_status": "unknown",
      "execution_status": "success",
      "warnings": [],
      "count": 1
    },
    "error": null
  }
}
```

## エラー時

エラー時は、AI Agentが読み取れる短い`content`、`isError: true`、機械処理用のエラーオブジェクトを同時に返します。

```json
{
  "content": [{"type": "text", "text": "dashboard repository not available"}],
  "structuredContent": {
    "data": null,
    "meta": {
      "schema_version": "1",
      "tool": "list_dashboards",
      "requested_at": "2026-10-12T09:00:00Z",
      "data_status": "error",
      "execution_status": "error",
      "warnings": []
    },
    "error": {
      "code": "TOOL_ERROR",
      "message": "dashboard repository not available",
      "retryable": false
    }
  },
  "isError": true
}
```

## フィールド

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `data` | any / `null` | ツールの結果本体。エラー時は`null`。 |
| `meta` | object | 共通メタ情報とツール固有のメタ情報。 |
| `meta.schema_version` | string | エンベロープ仕様のバージョン。現在は`"1"`。 |
| `meta.tool` | string | 呼び出されたMCPツール名。 |
| `meta.requested_at` | RFC3339 string | サーバーが処理を開始した時刻。 |
| `meta.data_status` | string | データの鮮度・性質。`live`、`historical`、`reference`、`degraded`、`unknown`など。エラー時は`error`。 |
| `meta.execution_status` | string | ツール処理の結果。`success`または`error`。 |
| `meta.warnings` | array | 警告。なければ空配列。 |
| `meta.input` | any（任意） | 秘密情報を除いた正規化済み入力。共通ラッパーは生のMCP引数をコピーしない。 |
| `error` | object / `null` | 成功時は`null`。エラー時は`code`、`message`、`retryable`を持つ。 |

ハンドラが返す`meta`はオブジェクトとしてマージされます。既存ハンドラとの互換性のため、オブジェクトでないメタ情報は`meta.details`に格納されます。共通フィールドはラッパーが設定します。

## 開発者向けの使い方

各モジュールは、`domain.Server.AddTool`に`(data, meta, err)`を返すハンドラを渡します。`mcp.CallToolResult`を直接生成する必要はありません。

```go
data := []map[string]any{{"id": "item-1"}}
server.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (any, any, error) {
	return data, map[string]any{
		"input": map[string]any{"limit": 20},
		"count": len(data),
	}, nil
})
```

データを返せない場合はエラーを返します。通常のエラーは`code: "TOOL_ERROR"`、`retryable: false`になります。エラー型に次のメソッドを実装すると、共通ラッパーでコードと再試行可否を指定できます。

```go
func (e *temporaryError) Code() string { return "UPSTREAM_UNAVAILABLE" }
func (e *temporaryError) Retryable() bool { return true }
```

`OutputSchema`をツール登録時に指定した場合、そのスキーマは自動的に`data`フィールドのスキーマとして共通エンベロープに組み込まれます。エラー時の`data: null`も許容されます。未指定の場合も、共通エンベロープの出力スキーマが自動設定されます。
