# pprotein-v2026

[kaz/pprotein](https://github.com/kaz/pprotein) を ISUCON に向けて手直しした版です。pprof のプロファイル、nginx のアクセスログ、MySQL のスロークエリログをまとめて取得し、ブラウザで見られるようにします。

元のコードの作者は Kazuki Sawada さん（[@kaz](https://github.com/kaz)）です。ライセンスは upstream と同じ MIT です（[LICENSE](LICENSE)）。

## upstream との違い

[kaz/pprotein@fd0b0b1](https://github.com/kaz/pprotein/tree/fd0b0b1da93994dc4abb986222b92606c4fe9b2b)（2024-11-07）を元にしています。エンドポイントは upstream と同じなので、upstream の `integration` を組み込んだアプリからもそのまま収集できます。

- Go のモジュールパスを `github.com/ymhiroki/pprotein-v2026` に変更
- サーバーを再起動するとメモが消えるバグを修正
- 複数の収集が同時に走ったときのデータ競合を修正（SSE の送信、pprof の Web UI のルート登録、google/pprof の内部状態、収集中のデータの JSON 化）
- alp / slp が失敗したとき、ツールのエラー出力も UI に表示
- clone した直後でも `make run` で起動できるように修正
- pprof / alp / slp / memo で重複していたハンドラを共通化

## pprotein サーバー

ビルドには Go 1.23.3 以上と Node.js（npm）が必要です。

```sh
make build
./pprotein
```

`:9000` で起動します。ポートは環境変数 `PORT` で変えられます。データはカレントディレクトリの `data/` に保存されます。

サーバーを動かす環境には、次のツールを入れておいてください。

- [alp](https://github.com/tkuchiki/alp): `go install github.com/tkuchiki/alp/cmd/alp@latest`
- [slp](https://github.com/tkuchiki/slp)（v0.2.0 以上）: `go install github.com/tkuchiki/slp/cmd/slp@latest`
- graphviz: pprof のグラフ表示に使います。フレームグラフだけなら要りません。

## 計測対象のアプリ

アプリに `/debug/*` のエンドポイントを追加します。echo v4 の場合は次のとおりです。

```go
import "github.com/ymhiroki/pprotein-v2026/integration/echov4"

echov4.Integrate(e)
```

`Integrate` は、echo の Debug モード、リクエストログ、Recover ミドルウェアも有効にします。リクエストログが負荷になる場合は、エンドポイントだけを追加する `echov4.EnableDebugHandler(e)` を使ってください。

gin、gorilla/mux、echo v3 用のパッケージもあります。ほかのフレームワークでは、`integration/standalone` の `go standalone.Integrate(":19000")` で別ポートに立てるのが簡単です。

アプリが動いていないホスト（DB サーバーなど）でログだけを取るときは、`pprotein-agent` を使います。`:19000` で起動します。

```sh
go install github.com/ymhiroki/pprotein-v2026/cli/pprotein-agent@latest
```

計測対象側は次の環境変数を読みます。

| 変数 | 既定値 | 内容 |
| --- | --- | --- |
| `PPROTEIN_HTTPLOG` | `/var/log/nginx/access.log` | nginx のアクセスログ（LTSV 形式） |
| `PPROTEIN_SLOWLOG` | `/var/log/mysql/mysql-slow.log` | MySQL のスロークエリログ |
| `PPROTEIN_GIT_REPOSITORY` | `.` | UI にコミット情報を出すための git リポジトリ |

nginx と MySQL の設定例は [mock/nginx.conf](mock/nginx.conf) と [mock/mysqld.cnf](mock/mysqld.cnf) にあります。MySQL は `long_query_time=0` にして、すべてのクエリを記録します。

## 収集

UI の setting ページで、`group/targets` に収集先を書きます。ホスト名とポートは環境に合わせて変えてください。

```json
[
  { "Type": "pprof", "Label": "isu1", "URL": "http://isu1:8080/debug/pprof/profile", "Duration": 60 },
  { "Type": "httplog", "Label": "isu1", "URL": "http://isu1:8080/debug/log/httplog", "Duration": 60 },
  { "Type": "slowlog", "Label": "isu2", "URL": "http://isu2:19000/debug/log/slowlog", "Duration": 60 }
]
```

- `Duration` は収集する秒数です。
- nginx 経由（80 番ポート）で取ると、`Duration` が nginx の `proxy_read_timeout`（既定 60 秒）を超えたときに失敗します。上の例のように、アプリのポートに直接つなぐのが確実です。
- pprof の URL を `/debug/fgprof` にすると、I/O 待ちの時間も含めたプロファイル（[fgprof](https://github.com/felixge/fgprof)）を取れます。
- `httplog/config`（alp の設定）の `matching_groups` は、デモ用の値になっています。アプリの URL に合わせて書き換えてください。

group ページの Collect ボタンか `GET /api/group/collect` で、すべての収集先から同時に取得します。`Duration` をベンチマークの長さに合わせ、ベンチマークの開始時にこの URL を叩くと、ベンチマーク中のデータだけを取れます。

pprotein サーバーにも `/debug/*` にも認証はありません。ポートは信頼できるネットワークにだけ公開してください。

## 開発

```sh
make run                    # UI が無ければビルドしてから、サーバーを起動
npm --prefix view run dev   # UI の開発サーバー。/api は 127.0.0.1:9000 に転送
```
