# dqd ticket new

`tickets/TICKET_ID.md` にチケットの雛形を作ります。

## 実行例

```sh
./bin/dqd -C todo-app ticket new -title "TODOの追加" -domains frontend TICKET-001
```

Makeで実行する場合は次の通りです。

```sh
make ticket-new PROJECT=todo-app TICKET_ID=TICKET-001 ARGS='-title "TODOの追加" -domains frontend'
```

| 引数・フラグ | 意味 |
| --- | --- |
| `TICKET_ID` | 必須。英字で始まる英数字・`-`・`_` |
| `-title TITLE` | 既定は「実装する機能の名前」 |
| `-domains LIST` | 技術領域。例 `frontend,backend` |
| `-include LIST` | 明示追加する登録済み品質ID。カンマ区切り |

`todo-app` は [init](init.md) で作成したプロジェクトです。成功時は相対パスを表示します。同名ファイルがある場合や、適用要求が0件の場合はエラーです。作成後に本文を編集します。[チケットの書式とAIへの依頼例](../tickets.md)

[コマンド一覧](README.md)
