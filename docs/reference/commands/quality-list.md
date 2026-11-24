# dqd quality list

登録済みの品質要求を宣言順に表示します。

## 実行例

```sh
./bin/dqd -C docs/example quality list
```

Makeで実行する場合は次の通りです。

```sh
make quality-list PROJECT=docs/example
```

| 引数・フラグ | 意味 |
| --- | --- |
| `-json` | JSONで出力。Makeでは `ARGS=-json` |

位置引数はありません。通常の列は `ID`、`NAME`、`DOMAIN`、`MODE` です。本文を読むには [quality show](quality-show.md) を使います。

[コマンド一覧](README.md)
