# ISUCON 当日の手順

pprotein-v2026 を ISUCON で使うための手順です。例年に多い、次の構成を前提にしています。

- サーバーは isu1〜isu3 の 3 台で、最初は isu1 に nginx、MySQL、アプリが全部載っている
- アプリは Go（echo）で、isucon ユーザーの systemd サービスとして動いている
- OS は Ubuntu

ホスト名、ユーザー名、ポート、サービス名は、当日の環境に合わせて読み替えてください。手順 1〜5、ベンチ、ログの truncate、pprotein-agent の流れは、Ubuntu 24.04（nginx 1.24、MySQL 8.0）のコンテナで、amd64 と arm64 の両方について確認しています。コンテナには systemd も SSH も無いので、`systemctl`、`scp`、`ssh` を使う部分は試していません。

## 前日までに: Linux 用のバイナリを作る

Go と Node.js が入った手元のマシンで作っておくと、当日はコピーするだけで済みます。

```sh
git clone https://github.com/ymhiroki/pprotein-v2026.git
cd pprotein-v2026
make view/dist
for arch in amd64 arm64; do
  GOOS=linux GOARCH=$arch CGO_ENABLED=0 go build -o ~/isucon-bin/linux-$arch/ ./cli/pprotein ./cli/pprotein-agent
done
```

## 当日の準備

### 1. pprotein を起動する

isu1 で `uname -m` を実行し、`x86_64` なら amd64、`aarch64` なら arm64 のバイナリと URL を使います。

```sh
# 手元から
scp ~/isucon-bin/linux-amd64/* isucon@isu1:/tmp/

# isu1 で
sudo install /tmp/pprotein /tmp/pprotein-agent /usr/local/bin/
curl -sSL https://github.com/tkuchiki/alp/releases/download/v1.0.22/alp_linux_amd64.tar.gz | sudo tar -xz -C /usr/local/bin alp
curl -sSL https://github.com/tkuchiki/slp/releases/download/v0.2.3/slp_linux_amd64.tar.gz | sudo tar -xz -C /usr/local/bin slp
mkdir -p ~/pprotein && cd ~/pprotein && nohup pprotein >> pprotein.log 2>&1 &
```

- slp は C のライブラリに依存しているので、手元からクロスビルドできません。配布されているバイナリを使います。動かすには glibc 2.34 以上（Ubuntu 22.04 以降）が必要です。
- データは `~/pprotein/data/` に保存されます。
- UI は、手元で `ssh -N -L 9000:localhost:9000 isucon@isu1` を実行してから http://localhost:9000 で開きます。認証が無いので、9000 番ポートは外部に開けないでください。

### 2. アプリに組み込む

アプリの `go.mod` があるディレクトリで次を実行します。

```sh
go get github.com/ymhiroki/pprotein-v2026@latest
```

コードには次を足します。

```go
import "github.com/ymhiroki/pprotein-v2026/integration/echov4"

echov4.EnableDebugHandler(e) // echo.New() のあと

// POST /initialize のハンドラの中に追加。ベンチと同時に収集が始まる
go func() {
	if resp, err := http.Get("http://localhost:9000/api/group/collect"); err == nil {
		resp.Body.Close()
	}
}()
```

- `/api/group/collect` はすぐに返ります。goroutine にしてあるのは、pprotein が止まっていても初期化処理を待たせないためです。
- echo 以外のフレームワークでは、`integration/standalone` を使って `go standalone.Integrate(":19000")` を入れます。その場合、手順 5 の URL のポートを 19000 にしてください。

### 3. nginx と MySQL のログを出す

nginx は、このリポジトリの [mock/nginx.conf](../mock/nginx.conf) にある `log_format ltsv` を http ブロックに写し、アクセスログの行を次のように変えます。変えたら `sudo nginx -t && sudo systemctl reload nginx` で再読み込みします。

```nginx
access_log /var/log/nginx/access.log ltsv;
```

MySQL は、`[mysqld]` に次の 3 行を足し、`sudo systemctl restart mysql` で再起動します。Ubuntu の MySQL 8.0 なら、設定ファイルは `/etc/mysql/mysql.conf.d/mysqld.cnf` です。

```ini
slow_query_log=ON
slow_query_log_file=/var/log/mysql/mysql-slow.log
long_query_time=0
```

### 4. アプリからログを読めるようにする

アプリは isucon ユーザーで動くので、そのままではログを読めません。Ubuntu 24.04 と MySQL 8.0 の標準の権限では、nginx のログを読むには adm グループが、スロークエリログを読むには adm と mysql の両方のグループが必要です。

```sh
sudo usermod -aG adm,mysql isucon && sudo systemctl restart <アプリのサービス名>
```

グループは、プロセスを起動し直したときに反映されます。

### 5. 収集先と alp の設定を書く

UI の setting ページで、`group/targets` を次のようにします。

```json
[
  { "Type": "pprof", "Label": "isu1", "URL": "http://localhost:8080/debug/pprof/profile", "Duration": 70 },
  { "Type": "httplog", "Label": "isu1", "URL": "http://localhost:8080/debug/log/httplog", "Duration": 70 },
  { "Type": "slowlog", "Label": "isu1", "URL": "http://localhost:8080/debug/log/slowlog", "Duration": 70 }
]
```

- 8080 はアプリのポートです。
- `Duration` は、マニュアルにある負荷走行の時間より少し長くします。収集は `/initialize` の時点で始まりますが、負荷は事前チェックのあとに始まることが多いためです。

同じページの `httplog/config` には、ID を含む URL を 1 行にまとめる正規表現を書きます。

```yaml
matching_groups:
  - ^/api/user/[^/]+$
```

設定を変えても、取得済みのデータは集計し直されません。最初のベンチの前に書いてください。

## ベンチのたびに

ポータルからベンチを実行すると、`/initialize` に足したコードで収集が自動で始まります。終わったら group ページを開きます。最新のグループが一番上に、開いた状態で表示されます。

- **httplog:** `Sum` 列をクリックしてソートし、合計時間の大きいエンドポイントから見ます。
- **slowlog:** `Sum(QueryTime)` でソートします。`Count` が極端に多いクエリは N+1 の可能性があります。
- **pprof:** フレームグラフが表示されます。CPU プロファイルには DB を待っている時間が出ないので、DB 待ちが長いアプリでは pprof の URL を `/debug/fgprof` に変えてください。
- **Commit 列:** どのコミットで測ったかを確認できます。
- **Add Memo:** スコアや変えたことを書いておくと、あとで比べやすくなります。

pprotein はベンチ中に増えた分だけを読むので、集計のためにログを毎回消す必要はありません。ただ、ログは溜まり続けますし、pprotein も毎回 `~/pprotein/data/` にコピーを保存します。ときどき `df -h` で空き容量を確認し、足りなくなったら、ベンチをしていない間にログを空にします。

```sh
sudo truncate -s 0 /var/log/nginx/access.log /var/log/mysql/mysql-slow.log
```

### 構成を変えたとき

- **DB を isu3 に移したとき:** 手順 1 と同じように isu3 にも pprotein-agent を入れて、`nohup sudo pprotein-agent >> agent.log 2>&1 &` で起動します。root で動くので、グループの設定は要りません。slowlog の URL は `http://<isu3 の IP>:19000/debug/log/slowlog` に変えます。
- **アプリを複数台にしたとき:** pprof の行を台数分に増やします。isu1 以外のサーバーで `/initialize` を受けるときは、手順 2 の URL の `localhost` を isu1 の IP に変えてください。

## 最後のベンチの前に

計測用の設定を外します。`long_query_time=0` のスロークエリログは全クエリを書き出すので、残したままだと負荷になります。

- MySQL を `slow_query_log=OFF` にし、nginx を `access_log off;` にする
- `/initialize` に足したコードを消す
- pprotein と pprotein-agent を止める

`EnableDebugHandler` で足した `/debug/*` は、呼ばれない限り何もしないので残しても構いません。

## うまくいかないとき

UI の Status 欄の「Failed …」をクリックすると、エラーの内容が表示されます。

- **`failed to open: ... permission denied`:** アプリがログを読めていません。手順 4 のあとにアプリを再起動したか確認してください。
- **`received empty content`:** `Duration` の間にログが 1 行も増えていません。ログのパス、nginx の再読み込み、MySQL の再起動を確認してください。
- **Commit 列にメッセージが出ない:** アプリの作業ディレクトリかその親が git リポジトリで、`origin` が設定されている必要があります。リポジトリが別の場所にあるなら、アプリの環境変数 `PPROTEIN_GIT_REPOSITORY` で指定します。
- **pprof の Graph が開けない:** graphviz が必要です（`sudo apt install graphviz`）。フレームグラフは graphviz なしで見られます。
