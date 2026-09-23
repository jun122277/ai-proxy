# M1: ここから自分で行う SRE の実装

次に着手するのは [M1-02: 通信のライフサイクル](https://github.com/jun122277/ai-proxy/issues/7)。メトリクス設計はその後の [M1-03](https://github.com/jun122277/ai-proxy/issues/8)。AI が準備した範囲は、モック upstream、固定 SDK の契約テスト、Compose、再現用の要求まで。

gateway 側のプロキシ、タイムアウト階層、キャンセル伝播、同時接続制限、readiness、SSE drain、メトリクスは未実装。M0 の HTTP プロセスの終了処理だけが既に存在する。

## まず確認するもの

リポジトリのルートで、Docker Desktop の Linux エンジンを起動しておく。

```powershell
docker compose -f deploy/compose/compose.yaml --profile mock up --build -d --wait gateway mockllm
curl.exe --fail-with-body -H "Content-Type: application/json" --data-binary "@tests/fixtures/chat.json" http://127.0.0.1:9090/v1/chat/completions
curl.exe --fail-with-body -N -H "Content-Type: application/json" --data-binary "@tests/fixtures/chat-stream.json" http://127.0.0.1:9090/v1/chat/completions
```

9090 はモック直結。8080 の gateway はまだ生成要求に 404 を返す。Linux/WSL では `curl.exe` を `curl` に読み替える。SDK 契約テストもモック直結であり、gateway 経由の試験はこれから追加する。

## 最初の設計メモ

実装前に以下を自分の言葉で埋め、`docs/adr/0003-stream-lifecycle.md` に判断を残す。この表は答えを固定しないため、未記入のままにしている。

| 判断すること | 自分の案・理由 |
| --- | --- |
| 接続・応答ヘッダー・最初の内容・チャンク間・全体の期限 | |
| 期限切れ時にクライアントへ何を返すか。送出前と後の違い | |
| クライアントが切断したとき、何をキャンセルするか | |
| 遅い受信者のために保持できるバッファ量と同時接続数 | |
| 終了開始後の新規要求と、処理中 SSE の扱い | |
| 生存と readiness の判定を分ける条件 | |

参考に読むコードは `cmd/gateway/main.go`、`internal/server/server.go`、`internal/config/config.go`。モックの挙動は [API 契約](../api-contract.md) を参照する。gateway コンテナからモックに接続する宛先は `http://mockllm:9090/v1/chat/completions`。

## 実装後に行う実験

最初の実験入力として、モックが最初の応答を3秒遅らせるようにする。3秒は故障入力であり、gateway の timeout の指定ではない。

```powershell
$env:MOCK_FIRST_EVENT_DELAY = '3s'
docker compose -f deploy/compose/compose.yaml --profile mock up -d --force-recreate mockllm
```

その後は**実装した gateway の 8080 に対して**要求を送り、選んだ deadline との関係を確認する。次に最初の遅延を戻し、内容の間隔を広げる。

```powershell
$env:MOCK_FIRST_EVENT_DELAY = '0s'
$env:MOCK_CHUNK_INTERVAL = '2s'
docker compose -f deploy/compose/compose.yaml --profile mock up -d --force-recreate mockllm
```

実験ごとに「予想 → 実測 → 差の原因 → 修正」を記録する。

- [ ] 最初の応答が遅い場合、選んだ期限とエラー表示が期待どおりか。
- [ ] SSE の途中でクライアントを止めた後、upstream と goroutine/接続が残り続けないか。
- [ ] 遅い受信者でもメモリが増え続けないか。
- [ ] 通信中に `docker compose -f deploy/compose/compose.yaml stop gateway` を実行し、正常終了と猶予超過を区別できるか。
- [ ] 認証、ボディサイズ、出力上限など、M1-02 の admission 条件をテストで確認したか。

HTTP 200 だけでは SSE 完了を判断できない。M1-03 では要求の正常完了、途中切断、キャンセル、最初の内容までの時間、active streams をどう数えるかを自分で決める。ここでは Grafana やメトリクスの実装を先回りして追加しない。

## 検証と後片付け

```powershell
docker compose -f deploy/compose/compose.yaml --profile tools run --build --rm dev make check vuln
docker compose -f deploy/compose/compose.yaml --profile mock down --remove-orphans
Remove-Item Env:MOCK_FIRST_EVENT_DELAY -ErrorAction SilentlyContinue
Remove-Item Env:MOCK_CHUNK_INTERVAL -ErrorAction SilentlyContinue
```

Actions は作業中に起動しない。ローカルで検証し、最終 commit の Draft PR を ready にしたときだけ必須の Go チェックを1回実行する。
