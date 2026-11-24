# DQD コマンドリファレンス

| DQD | Make | 用途 |
| --- | --- | --- |
| [`init`](init.md) | `make init` | 新しいプロジェクトを作ります。 |
| [`validate`](validate.md) | `make validate` | 宣言とファイルの整合性を調べます。 |
| [`ticket new`](ticket-new.md) | `make ticket-new` | `tickets/TICKET_ID.md` にチケットの雛形を作ります。 |
| [`resolve`](resolve.md) | `make resolve` | チケットに適用する品質要求と選ばれた理由を表示します。 |
| [`context`](context.md) | `make context` | 機能要件、品質要求、個別合格条件をMarkdownにまとめます。 |
| [`verify`](verify.md) | `make verify` | チケットに適用する品質IDのテストを実行します。 |
| [`report`](report.md) | `make report` | [verify](verify.md) の保存結果をHTML・テキスト・JSONで表示します。 |
| [`quality list`](quality-list.md) | `make quality-list` | 登録済みの品質要求を宣言順に表示します。 |
| [`quality show`](quality-show.md) | `make quality-show` | 指定した品質IDの見出しとMarkdown本文を表示します。 |

## 共通の指定

例はDQDリポジトリのルートから実行します。[ビルド手順](../../../README.md#makeなしで使う)

| 指定 | 意味 |
| --- | --- |
| `-C DIR` / `PROJECT=DIR` | 対象プロジェクト。既定は現在のディレクトリ。`init` では作成先の親 |
| `TICKET_ID` | Makeで指定するチケットID。既定は `TICKET-001` |
| `QUALITY_ID` | Makeで指定する品質ID。既定は `UI-001` |
| `ARGS` | Makeから渡す追加フラグ。例 `ARGS=-details` |

`-C` は `dqd -C DIR quality show ID` または `dqd quality show -C DIR ID` のように指定します。フラグはチケットIDなどの位置引数より前に置きます。

```sh
./bin/dqd -h
./bin/dqd report -h
make dqd ARGS="report -h"
```

CLIの終了コードは成功・ヘルプが `0`、処理エラーや `verify` が全件合格しなかったときが `1`、コマンド・引数エラーが `2` です。Make経由ではMakeの終了コードが返る場合があります。JSONを別のツールに渡す場合は、実行ファイルを直接呼び出します。
