# `internal/myherdr/web/`

localhost server が配信し、Go binary に埋め込む端末 UI です。端末出力は xterm.js が描画し、DOM HTML として挿入しません。

## Index

| パス | 役割 |
| --- | --- |
| [`README.md`](./README.md) | Web 資材の入口。 |
| [`LICENSES.md`](./LICENSES.md) | 同梱する端末表示ライブラリの出所とライセンス。 |
| [`app.js`](./app.js) | pane 選択、WebSocket 接続、terminal 入力・resize。 |
| [`index.html`](./index.html) | 端末画面の HTML。 |
| [`style.css`](./style.css) | 画面スタイル。 |
| [`vendor/`](./vendor/) | xterm.js と fit addon の配布資材・ライセンス。 |
