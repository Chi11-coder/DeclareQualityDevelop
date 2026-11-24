# dqd resolve

チケットに適用する品質要求と選ばれた理由を表示します。

## 実行例

```sh
./bin/dqd -C docs/example resolve TICKET-001
```

Makeで実行する場合は次の通りです。

```sh
make resolve PROJECT=docs/example TICKET_ID=TICKET-001
```

| 引数・フラグ | 意味 |
| --- | --- |
| `TICKET_ID` | 必須。対象チケットのID |
| `-json` | JSONで出力。Makeでは `ARGS=-json` |

標準出力の列は `ID`、`NAME`、`REASON` です。理由は `always`（常時）、`attribute: 属性名`（属性一致）、`include`（明示指定）。表示は宣言順です。[適用条件](../requirements.md#適用条件)

[コマンド一覧](README.md)
