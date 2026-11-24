# dqd verify

チケットに適用する品質IDのテストを実行します。

## 実行例

```sh
./bin/dqd -C todo-app verify -timeout=5m TICKET-001
```

Makeで実行する場合は次の通りです。

```sh
make verify PROJECT=todo-app TICKET_ID=TICKET-001 ARGS=-timeout=5m
```

| 引数・フラグ | 意味 |
| --- | --- |
| `TICKET_ID` | 必須。対象チケットのID |
| `-timeout DURATION` | チェック1件の制限時間。既定は `2m` |

[verification.yml](../verification.md) にコマンドを登録します。結果とログは `todo-app/verifies/TICKET-001.json` に保存されます。チェック未登録の要求は未確認です。

結果を保存できるか、テストの起動前に確認します。タイムアウトや中断時は、チェックと同じプロセスグループ（Unix系）またはJob（Windows）に属する子プロセスも終了します。

全要件が合格すれば終了コード `0`、不合格・未確認・実行エラーがあれば結果を保存して `1` を返します。保存後は [report](report.md) で確認できます。

[コマンド一覧](README.md)
