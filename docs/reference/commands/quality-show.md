# dqd quality show

指定した品質IDの見出しとMarkdown本文を表示します。

## 実行例

```sh
./bin/dqd -C docs/example quality show UI-001
```

Makeで実行する場合は次の通りです。

```sh
make quality-show PROJECT=docs/example QUALITY_ID=UI-001
```

| 引数・フラグ | 意味 |
| --- | --- |
| `ID` | 必須。登録済みの品質ID |

未登録のIDはエラーです。IDを調べるには [quality list](quality-list.md) を使います。

[コマンド一覧](README.md)
