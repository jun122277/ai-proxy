# M1 のモック API 契約

対象は `mockllm` の `POST /v1/chat/completions`。OpenAI Go SDK **v3.66.0** で検証する、テキスト応答に限定したローカル fixture である。gateway の中継、認証、deadline、SSE drain を実装済みとは扱わない。

## 受け付ける要求

Content-Type は `application/json`。ボディは64 KiB 以下の単一 JSON オブジェクト。不明フィールドは 400、過大なボディは 413、非対応 Content-Type は 415 とする。これらは fixture の制限であり、gateway の admission 方針は M1-02 で設計する。

| フィールド | 対応範囲 |
| --- | --- |
| `model` | 必須。`mock-model` のみ |
| `messages` | 必須。1件以上。各要素は `role` と文字列の `content` |
| `messages[].role` | `system` / `developer` / `user` / `assistant` |
| `stream` | 省略時 false。true で SSE |
| `max_completion_tokens` | 任意。1〜4096。後述の合成出力単位に対する上限 |
| `stream_options.include_usage` | `stream=true` 時のみ指定可。true で最後に usage イベントを付ける |

tools、画像、音声、構造化出力、`temperature`、旧 `max_tokens`、Responses API は対象外。実モデルやトークナイザーを再現せず、プロンプトの内容にも依存しない。

## 応答

通常応答は `chat.completion`、assistant の固定文 `Hello from mock.` を返す。SSE の正常系は次の順序とする。

1. `chat.completion.chunk` の `delta.role=assistant`。
2. `delta.content` が `Hello`、` from`、` mock`、`.` の4イベント。
3. 空の delta と `finish_reason=stop`。
4. 要求された場合のみ、`choices=[]` と usage を持つイベント。
5. `data: [DONE]`。

各イベントは `data: <JSON>\n\n` 形式で送信して flush する。同一応答の ID、model、created はイベント間で共通。通常チャンクの usage は null。最初の role イベントと最初の内容トークンは別物として計測する。

usage は **入力1メッセージ=1、出力1断片=1** の合成値。通常は出力4単位であり、課金量ではない。例えば `max_completion_tokens=2` なら `Hello from`、出力2単位、`finish_reason=length` となる。クォータの実装や厳密なトークン保証は含まない。

エラーは `{"error":{"message":"...","type":"invalid_request_error","param":null,"code":null}}` の形で返す。入力値をエラーやログにそのまま出さない。モックに認証機能はなく、SDK 用の `mock-test-key` は単なるダミー値。

## 遅延の入力

CLI の `-first-event-delay` と `-chunk-interval` は0〜1分。前者はヘッダー/最初のイベントの前、後者は各内容断片の前で待つ。通常 JSON 応答に使うのは前者だけ。

Compose の `mock` profile では `MOCK_FIRST_EVENT_DELAY`（既定0s）、`MOCK_CHUNK_INTERVAL`（既定100ms）で変更する。通常の healthcheck には遅延を適用しない。これは gateway のタイムアウト設計を試すための入力であり、選ぶべき timeout 値を示してはいない。

モック自身は接続が切れると待機を終了する。gateway を間に置いた場合にキャンセルが伝播するかは、これから利用者が実装・検証する。

## 契約テストの範囲

`tests/sdk` は独立した Go module で、SDK バージョンと依存関係を `go.mod` / `go.sum` に固定する。実行時は `httptest` のローカルリスナーにのみ接続し、transport で他の宛先を拒否する。実 API キーと実 API 呼び出しは不要。初回の依存パッケージ取得にはネットワーク接続が必要。

`make test-sdk` は通常応答、SSE、usage 有無、出力上限、SDK でのエラー復元を確認する。SDK はテスト用 module にのみ依存し、gateway/mockllm の実行バイナリにはリンクしない。

形式の参照元: [OpenAI Chat Completions](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)、[Streaming API responses](https://developers.openai.com/api/docs/guides/streaming-responses)。対応範囲はこの文書の表に限定する。
