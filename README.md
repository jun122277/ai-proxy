# ai-proxy

外部 API の障害と長寿命の Streaming 接続を題材に、信頼性の設計・計測・障害対応・安全なリリースを実証する SRE ポートフォリオ。

M0 の基盤を実装・検証済みです。現在は設定検証と HTTP プロセスのヘルスチェックまで動作します。LLM プロキシ、認証、SSE、`/readyz` は M1 で追加します。

実装範囲とデモの合格条件は [ロードマップ](docs/ROADMAP.md)、初期判断は [ADR-0001](docs/adr/0001-sre-portfolio-scope.md) を参照してください。

## ローカル起動

必要なものは Linux コンテナを実行できる Docker Engine/Desktop と Compose v2 以降です。Windows の PowerShell でも以下のコマンドを使えます。初回はインターネット接続とイメージのダウンロードが必要です。

```sh
docker compose -f deploy/compose/compose.yaml up --build -d --wait gateway
curl http://127.0.0.1:8080/healthz
docker compose -f deploy/compose/compose.yaml down
```

PowerShell では `curl.exe http://127.0.0.1:8080/healthz` を使ってください。期待する応答は `{"status":"ok"}` です。`POST /v1/chat/completions` はまだ 404 を返します。

既定の公開先は `127.0.0.1:8080` です。ポートが使用中の場合は `GATEWAY_PORT` を変更します。PowerShell では `$env:GATEWAY_PORT = '18080'` としてから起動します。

Make を使用できる Linux/WSL 環境では `make up` / `make smoke` / `make down` も使えます。gateway コンテナは CPU 1、メモリ128 MiB を上限にします。開発用ビルドを含む検証環境は Docker への割当8 CPU・約4 GiB メモリで確認しています。これは最低要件や負荷試験結果ではありません。

## 開発と検証

ホストに Go/Make をインストールせずに、固定した開発用コンテナで検証できます。

```sh
docker compose -f deploy/compose/compose.yaml --profile tools run --build --rm dev make check vuln
```

Go 1.27.1 と Make がある場合は以下を実行します。

```sh
make check  # gofmt の検査、go vet、go test -race、ビルド
make vuln   # govulncheck v1.8.0
make run
```

整形は `make fmt`、実行イメージの HIGH/CRITICAL 脆弱性検査は `make scan` です。`make scan` には Docker とネットワーク接続が必要です。開発用コンテナには Docker ソケットを渡していないため、`make up` / `make scan` はホストまたは CI で実行してください。

イメージは digest、CI action は commit に固定しています。実 API の呼び出しや API キーは不要です。[M0 の検証記録](docs/reports/m0-validation.md) も参照してください。

## Actions の使用量を抑える運用

[GitHub Actions](https://github.com/jun122277/ai-proxy/actions/workflows/ci.yml) は手動実行のみです。push、PR 更新、main への取り込み、定期スケジュールでは実行しません。通常の検証はローカルで済ませ、PR の最終 commit に対して Go チェックを1回実行します。実行後に commit を追加した場合は、その新しい commit のチェックが必要です。

```sh
gh workflow run ci.yml --ref <PRのブランチ名>
```

既定は format/vet、race test、build、設定例の確認で、1ジョブ・最大5分です。main の必須チェックは `Go checks` とし、コンテナと脆弱性検査はローカルの結果を PR に記録します。必要な場合だけ次のフル CI を明示的に実行します。

```sh
gh workflow run ci.yml --ref <PRのブランチ名> -f full=true
```

フル CI では govulncheck とコンテナ検査・Trivy を追加し、コンテナ側の上限は10分です。上限時間は最大値であり、実行時間や料金の見積もりではありません。初回のフル検証は [実行記録](https://github.com/jun122277/ai-proxy/actions/runs/35861371164) に残しています。

## 設定と終了処理

[設定例](config/gateway.example.json) は非秘密の JSON です。コンテナ用の設定は [gateway.container.json](config/gateway.container.json) をイメージに同梱します。

```sh
go run ./cmd/gateway -config config/gateway.example.json -check-config
```

設定を省略した場合だけ既定値を使い、指定ファイルが読めない場合や不正な値がある場合は起動に失敗します。不明キー、重複キー、null、追加 JSON、64 KiB 超のファイルを拒否し、エラーに設定値を含めません。将来の API キーも設定ファイルには保存しません。

SIGTERM/割込み時は受付を停止し、処理中の要求を `shutdown_timeout` まで待ちます。期限切れでは接続を閉じ、非ゼロで終了します。M0 の `/healthz` は生存確認のみです。SSE と Kubernetes の drain は後続マイルストーンで検証します。

## 開発状況

| 段階 | 状態 |
| --- | --- |
| M0 Go・コンテナ・CI・開発運用 | 実装・検証済み |
| M1 プロキシと可観測性 | 未着手 |
| M2〜M7 SLO・障害対応・Kubernetes・GitOps | 未着手 |

[Milestones](https://github.com/jun122277/ai-proxy/milestones) / [Issues](https://github.com/jun122277/ai-proxy/issues) / [開発手順](CONTRIBUTING.md) / [セキュリティ報告](SECURITY.md) / [MIT License](LICENSE)
