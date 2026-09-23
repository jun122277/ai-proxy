# M1-01 準備の検証記録

実施日: 2026-09-23。対象はモック upstream と SDK 契約。M1-02/M1-03 の SRE 実装を完了したことを示す記録ではない。

| 検証 | 結果 |
| --- | --- |
| Go format / vet | root module と SDK テスト module で成功 |
| `go test -race` | 既存パッケージ、mock handler、SDK 契約で成功 |
| 通常の JSON 応答 | 固定の assistant 応答と合成 usage を確認 |
| SSE の wire 契約 | role、4断片、finish、任意の usage、DONE の順序を確認 |
| incremental delivery | fixture の全応答を待たず最初のイベントを受け取り、接続切断後に fixture の待機が終わることを確認 |
| 非対応入力 | 不明フィールド、非テキスト入力、不正な model/role/上限、過大ボディを拒否 |
| OpenAI Go SDK v3.66.0 | 通常応答、usage 有無の SSE、出力上限、API エラーをローカル HTTP 経由で確認 |
| 静的ビルド / govulncheck | gateway/mockllm をビルド、root module で到達可能な脆弱性の検出なし |
| Compose | gateway/mockllm とも healthy まで起動 |
| モックのコンテナ応答 | 9090 で JSON、SSE、usage、DONE を実際に受信 |
| gateway の未実装境界 | 8080 の生成 API は引き続き404 |

環境は Go 1.27.1 / Linux amd64 の開発用コンテナと Docker Desktop。SDK は `tests/sdk/go.mod` / `go.sum` に固定し、runtime の依存関係には追加していない。SDK テスト用 transport はそのテストのローカルリスナー以外への接続を拒否する。

再現: `docker compose -f deploy/compose/compose.yaml --profile tools run --build --rm dev make check vuln`。実際のコンテナへ送る要求は `tests/fixtures/` と [着手ガイド](../exercises/m1-lifecycle.md) を参照。

gateway を経由する切断伝播、タイムアウト、過負荷制御、readiness、SSE drain、メトリクスは未検証・未実装。利用者が次に担当する。
