# M0 検証記録

実施日: 2026-09-23。M0 のプロセス基盤を対象とする。SSE、可用性 SLO、実 API、Kubernetes は未検証。

## 環境

| 項目 | 値 |
| --- | --- |
| ホスト | Windows / PowerShell |
| Docker Desktop / Engine | 4.90.0 / 29.7.2、Linux amd64 |
| Compose | 5.5.1 |
| Docker 割当 | 8 CPU、4,028,235,776 bytes RAM |
| Go | 1.27.1、固定 digest の開発用コンテナ |
| govulncheck | v1.8.0 |
| Trivy | 0.74.0、固定 digest |
| Runtime | distroless static-debian13、UID/GID 65532 |

## 実行内容と結果

| 検証 | 結果 |
| --- | --- |
| `make check` 相当の format/vet/race/build | 3パッケージ成功 |
| 不正設定、重複/未知キー、null、過大ファイル、値の秘匿 | 拒否とエラー表示を単体テストで確認 |
| 処理中の要求を待つ終了 | 応答完了まで待機し、内容を失わないことをテスト |
| 終了猶予を超過する要求 | 接続キャンセルと deadline エラーをテスト |
| `make vuln` | 到達可能な脆弱性の検出なし |
| Compose `up --build -d --wait` | healthy まで起動 |
| `/healthz` | HTTP 200、`{"status":"ok"}` |
| 未実装の `POST /v1/chat/completions` | HTTP 404 |
| コンテナ制限 | UID/GID 65532:65532、ReadonlyRootfs=true |
| Compose stop | `gateway shutdown complete`、終了コード0 |
| Trivy runtime image scan | Debian/Go binary とも HIGH/CRITICAL の検出0件 |
| 設定例の `-check-config` | 終了コード0 |

再現コマンドは [README](../../README.md)、Linux で同じチェックを行う定義は [CI](../../.github/workflows/ci.yml) に置く。

## GitHub Actions

[初回のフル CI](https://github.com/jun122277/ai-proxy/actions/runs/35861371164) は commit `cfaa169b546cb9119062b419775bac584d618196` に対して成功した。Go checks と Container checks の両方を含む。対象の実装は [PR #16](https://github.com/jun122277/ai-proxy/pull/16) から追跡できる。

以降は利用枠を節約するため手動実行のみとする。通常は Go チェックの1ジョブ、フル CI は明示的な `full=true` 指定時に限る。PR 作成や main への取り込みで自動実行しない。

## 検証中に見つかった問題

GitHub Releases の latest が返した govulncheck v1.1.4 は Go 1.27 の構文解析で panic した。公式リポジトリのタグと依存関係を確認し、v1.8.0 に更新して正常終了を確認した。解析の skip や失敗の握り潰しは行っていない。

## 制限

これはローカルでの機能検証であり、最小ハードウェア要件、負荷限界、長期 SLO、無停止更新を示す結果ではない。脆弱性検査の結果は実施時点の DB と対象 severity に依存する。
