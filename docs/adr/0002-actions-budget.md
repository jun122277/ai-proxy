# ADR-0002: Actions を明示的な最終確認に限定する

- 状態: Accepted
- 日付: 2026-09-23
- 更新対象: ADR-0001 の CI 実行方針

## 背景

利用できる GitHub Actions の枠が少ないため、実行回数と重複ビルドを抑える必要がある。M0 の Go/コンテナ両方の CI は一度成功しており、ローカルでも同じ検証を実行できる。

## 決定

PR は Draft で作成し、最終 commit を push した後に Draft を解除する。`pull_request: types: [ready_for_review]` でこの操作だけを CI の起点にする。push、PR の作成・更新、main への取り込み、スケジュールを起点とする自動 CI は設定しない。

通常はローカルで検証を済ませ、マージする最終 commit に対して Go チェックを1回実行する。main は PR と `Go checks` の成功を引き続き必須とする。検証後に commit を追加した場合は Draft に戻し、再び ready にして検証する。

コンテナと脆弱性検査はローカルを基本とし、必要な場合だけ `full=true` でリモートでも実行する。通常の1ジョブは最大5分、任意のコンテナジョブは最大10分で停止する。

`workflow_dispatch` のチェックは PR の必須チェックを満たさないことを実機と公式資料で確認した。そのため Run workflow ボタンを必須チェックの入口には使わず、追加検証用に限定する。[GitHub: Troubleshooting required status checks](https://docs.github.com/en/pull-requests/how-tos/merge-and-close-pull-requests/troubleshooting-required-status-checks)

## トレードオフ

各更新に対する即時フィードバックと、毎回の独立したコンテナ再検証は減る。代わりにローカルの検証結果を PR に記録し、最終 commit の Go チェックは維持する。利用枠が増えたら、自動実行の範囲を再評価する。
