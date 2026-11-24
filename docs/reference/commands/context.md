# dqd context

機能要件、品質要求、個別合格条件をMarkdownにまとめます。

## 実行例

```sh
./bin/dqd -C docs/example context -stdout TICKET-001
```

Makeで実行する場合は次の通りです。

```sh
make context PROJECT=docs/example TICKET_ID=TICKET-001 ARGS=-stdout
```

| 引数・フラグ | 意味 |
| --- | --- |
| `TICKET_ID` | 必須。対象チケットのID |
| `-stdout` | 保存せず端末に表示 |
| `-force` | 保存済みのContextを上書き |

`-stdout` を省略すると、プロジェクト内の `.dqd/contexts/TICKET_ID.md` に保存して相対パスを表示します。生成結果をAIへ渡します。

[コマンド一覧](README.md)
