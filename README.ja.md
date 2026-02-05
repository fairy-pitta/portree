# portree - Git Worktree Server Manager

[![CI](https://github.com/fairy-pitta/portree/actions/workflows/ci.yaml/badge.svg)](https://github.com/fairy-pitta/portree/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/fairy-pitta/portree/branch/main/graph/badge.svg)](https://codecov.io/gh/fairy-pitta/portree)
[![Go Report Card](https://goreportcard.com/badge/github.com/fairy-pitta/portree)](https://goreportcard.com/report/github.com/fairy-pitta/portree)

**portree** は [git worktree](https://git-scm.com/docs/git-worktree) ごとに複数の dev server を自動管理する CLI ツールです。ポートの自動割り当て、環境変数の自動注入、`*.localhost` サブドメインルーティングによるリバースプロキシを提供します。

> English version: [README.md](./README.md)

---

## 特徴

- **マルチサービス** — フロントエンド、バックエンド、任意の数のサービスを worktree ごとに定義
- **ポート自動割り当て** — ハッシュベース (FNV32) のポート割り当て。worktree 間のポート衝突なし
- **サブドメインリバースプロキシ** — `branch-name.localhost:<port>` で任意の worktree にアクセス (`/etc/hosts` の編集不要)
- **環境変数の自動注入** — `$PORT`、`$PT_BRANCH`、`$PT_BACKEND_URL` 等を自動設定
- **TUI ダッシュボード** — ターミナル上のインタラクティブ UI でサービスの起動・停止・監視
- **プロセスライフサイクル管理** — グレースフルシャットダウン (SIGTERM → SIGKILL)、ログファイル、古い PID の自動クリーンアップ
- **worktree ごとのオーバーライド** — ブランチ別にコマンド、ポート、環境変数をカスタマイズ

---

## クイックスタート

### 1. インストール

```bash
# ソースから
go install github.com/fairy-pitta/portree@latest

# またはローカルビルド
git clone https://github.com/fairy-pitta/portree.git
cd portree
make build
```

### 2. 初期化

```bash
cd your-project
portree init
# リポジトリルートに .portree.toml を作成
```

### 3. 設定

`.portree.toml` をプロジェクトに合わせて編集:

```toml
[services.frontend]
command = "pnpm run dev"
dir = "frontend"
port_range = { min = 3100, max = 3199 }
proxy_port = 3000

[services.backend]
command = "source .venv/bin/activate && python manage.py runserver 0.0.0.0:$PORT"
dir = "backend"
port_range = { min = 8100, max = 8199 }
proxy_port = 8000

[env]
NODE_ENV = "development"
```

### 4. サービス起動

```bash
portree up            # 現在の worktree の全サービスを起動
portree up --all      # 全 worktree の全サービスを起動
```

### 5. プロキシ起動

```bash
portree proxy start
# :3000 → frontend サービス
# :8000 → backend サービス
```

### 6. ブラウザで開く

```bash
portree open                    # http://main.localhost:3000 を開く
portree open --service backend  # http://main.localhost:8000 を開く
```

---

## コマンド一覧

| コマンド               | 説明                                             |
| ---------------------- | ------------------------------------------------ |
| `portree init`         | `.portree.toml` 設定ファイルを作成               |
| `portree up`           | 現在の worktree のサービスを起動                 |
| `portree up --all`     | 全 worktree のサービスを起動                     |
| `portree up --service` | 特定のサービスのみ起動                           |
| `portree down`         | 現在の worktree のサービスを停止                 |
| `portree down --all`   | 全 worktree のサービスを停止                     |
| `portree ls`           | 全 worktree のサービス、ポート、状態、PID を表示 |
| `portree dash`         | インタラクティブ TUI ダッシュボードを起動        |
| `portree proxy start`  | リバースプロキシを起動 (フォアグラウンド)        |
| `portree proxy stop`   | リバースプロキシを停止                           |
| `portree open`         | 現在の worktree をブラウザで開く                 |
| `portree version`      | バージョン情報を表示                             |

---

## 設定リファレンス

`.portree.toml` は git リポジトリのルートに配置します。

### `[services.<name>]`

1 つ以上のサービスを定義します。各 worktree で定義された全サービスが起動されます。

| フィールド   | 型           | 必須   | 説明                                               |
| ------------ | ------------ | ------ | -------------------------------------------------- |
| `command`    | string       | はい   | サービスを起動するシェルコマンド                   |
| `dir`        | string       | いいえ | worktree ルートからの相対パス (デフォルト: ルート) |
| `port_range` | `{min, max}` | はい   | このサービスのポート割り当て範囲                   |
| `proxy_port` | int          | はい   | リバースプロキシがリッスンするポート               |

```toml
[services.frontend]
command = "pnpm run dev"
dir = "frontend"
port_range = { min = 3100, max = 3199 }
proxy_port = 3000
```

### `[env]`

全サービスに注入されるグローバル環境変数。

```toml
[env]
NODE_ENV = "development"
DATABASE_URL = "postgres://localhost/mydb"
```

### `[worktrees."<branch>"]`

worktree ごとのオーバーライド。コマンド、固定ポート、追加環境変数をカスタマイズできます。

```toml
[worktrees.main]
services.frontend.port = 3100       # main ブランチのポートを固定

[worktrees."feature/auth"]
services.backend.command = "python manage.py runserver --settings=myapp.auth 0.0.0.0:$PORT"
services.backend.env = { DEBUG = "1" }
```

---

## 環境変数

portree は以下の環境変数を全サービスプロセスに自動注入します:

| 変数                | 例                                                  | 説明                                     |
| ------------------- | --------------------------------------------------- | ---------------------------------------- |
| `PORT`              | `3117`                                              | このサービスの割り当てポート             |
| `PT_BRANCH`         | `feature/auth`                                      | 現在のブランチ名                         |
| `PT_BRANCH_SLUG`    | `feature-auth`                                      | ブランチ名の URL-safe スラッグ           |
| `PT_SERVICE`        | `frontend`                                          | 現在のサービス名                         |
| `PT_<SERVICE>_PORT` | `PT_FRONTEND_PORT=3117`                             | 同一 worktree の各サービスのポート       |
| `PT_<SERVICE>_URL`  | `PT_BACKEND_URL=http://feature-auth.localhost:8000` | 同一 worktree の各サービスのプロキシ URL |

これにより、サービス間の通信設定を自動解決できます:

```js
// next.config.js
module.exports = {
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${process.env.PT_BACKEND_URL}/api/:path*`,
      },
    ];
  },
};
```

---

## 仕組み

```
┌─────────────────────────────────────────────────────────────┐
│  git リポジトリ                                              │
│                                                             │
│  main worktree          feature/auth worktree               │
│  ┌───────────────┐      ┌───────────────┐                   │
│  │ frontend :3100│      │ frontend :3117│                   │
│  │ backend  :8100│      │ backend  :8104│                   │
│  └───────────────┘      └───────────────┘                   │
│         │                      │                            │
└─────────┼──────────────────────┼────────────────────────────┘
          │                      │
    ┌─────▼──────────────────────▼─────┐
    │     portree リバースプロキシ      │
    │                                  │
    │  :3000  ←  *.localhost:3000      │
    │  :8000  ←  *.localhost:8000      │
    └──────────────────────────────────┘
          │                      │
          ▼                      ▼
  main.localhost:3000    feature-auth.localhost:3000
  main.localhost:8000    feature-auth.localhost:8000
```

1. **ポート割り当て** — `FNV32(branch:service) % range` でポートを決定。再起動しても安定。
2. **プロセス管理** — サービスはプロセスグループ付きの子プロセスとして実行。ログは `.portree/logs/` に出力。
3. **リバースプロキシ** — `proxy_port` ごとに HTTP リスナーを起動。`Host` ヘッダーのサブドメインでルーティング。
4. **`*.localhost`** — [RFC 6761](https://tools.ietf.org/html/rfc6761) により、モダンブラウザは `*.localhost` を `127.0.0.1` に自動解決。DNS 設定不要。

---

## TUI ダッシュボード

`portree dash` で起動:

```
╭─ portree dashboard ──────────────────────────────────────────╮
│                                                               │
│  WORKTREE        SERVICE    PORT   STATUS      PID            │
│  ──────────────────────────────────────────────────────────── │
│ ▸ main           frontend   3100   ● running   12345          │
│   main           backend    8100   ● running   12346          │
│   feature/auth   frontend   3117   ○ stopped   —              │
│   feature/auth   backend    8104   ○ stopped   —              │
│                                                               │
│  Proxy: ● running (:3000, :8000)                              │
│                                                               │
│  [s] start  [x] stop  [r] restart  [o] open in browser       │
│  [a] start all  [X] stop all  [p] toggle proxy                │
│  [l] view logs  [q] quit                                      │
╰───────────────────────────────────────────────────────────────╯
```

**キーバインド:**

| キー    | 操作                     |
| ------- | ------------------------ |
| `j`/`k` | カーソル移動 (下/上)     |
| `s`     | 選択中のサービスを起動   |
| `x`     | 選択中のサービスを停止   |
| `r`     | 選択中のサービスを再起動 |
| `o`     | ブラウザで開く           |
| `a`     | 全サービス起動           |
| `X`     | 全サービス停止           |
| `p`     | プロキシの切り替え       |
| `l`     | ログファイルパスを表示   |
| `q`     | 終了                     |

---

## 使用例

```bash
# フロントエンド + バックエンドのモノレポで作業中
cd my-project

# portree を初期化
portree init
# .portree.toml を編集してサービスを定義...

# フィーチャーブランチの worktree を作成
git worktree add ../my-project-feature-auth feature/auth

# 現在のブランチのサービスを起動
portree up
# Starting frontend (port 3100) for main ...
# Starting backend (port 8100) for main ...
# ✓ 2 services started for main

# 全 worktree のサービスを一括起動
portree up --all
# ✓ 4 services started

# 状態確認
portree ls
# WORKTREE        SERVICE    PORT   STATUS    PID
# main            frontend   3100   running   12345
# main            backend    8100   running   12346
# feature/auth    frontend   3117   running   12347
# feature/auth    backend    8104   running   12348

# プロキシ起動
portree proxy start
# アクセス:
#   http://main.localhost:3000          → frontend (main)
#   http://main.localhost:8000          → backend (main)
#   http://feature-auth.localhost:3000  → frontend (feature/auth)
#   http://feature-auth.localhost:8000  → backend (feature/auth)

# ブラウザで開く
portree open
# Opening http://main.localhost:3000 ...

# TUI を使う
portree dash

# 終了時
portree down --all
# ✓ 4 services stopped
```

---

## FAQ

### `*.localhost` は全てのブラウザで動きますか？

Chrome、Firefox、Edge、Safari などのモダンブラウザは [RFC 6761](https://tools.ietf.org/html/rfc6761) に従い `*.localhost` を `127.0.0.1` に解決します。`/etc/hosts` の編集や DNS 設定は不要です。

### 2 つの worktree が同じポートにハッシュされた場合は？

portree は linear probing を使用します。ハッシュで決まったポートが使用中の場合、範囲内の次の空きポートを探します。

### プロキシなしで使えますか？

はい。`portree up` でサービスを起動すれば、`localhost:<port>` で直接アクセスできます。プロキシはオプションです。

### ログはどこに保存されますか？

サービスのログは main worktree のルート配下の `.portree/logs/<branch-slug>.<service>.log` に書き出されます。

### 状態はどこに保存されますか？

ランタイム状態 (PID、ポート割り当て) は `.portree/state.json` に保存され、ファイルロックで同時アクセスの安全性を確保しています。

### ブランチごとに異なるコマンドを実行できますか？

はい。`.portree.toml` の `[worktrees."branch-name"]` でオーバーライドできます:

```toml
[worktrees."feature/auth"]
services.backend.command = "python manage.py runserver --settings=auth 0.0.0.0:$PORT"
services.backend.env = { DEBUG = "1" }
```

---

## コントリビューション

1. リポジトリをフォーク
2. フィーチャーブランチを作成 (`git checkout -b feature/amazing`)
3. 変更をコミット (`git commit -m 'feat: add amazing feature'`)
4. ブランチをプッシュ (`git push origin feature/amazing`)
5. Pull Request を作成

```bash
# 開発
make build      # バイナリをビルド
make test       # レースディテクタ付きでテスト実行
make lint       # golangci-lint を実行
make all        # fmt + vet + lint + test + build
```

---

## ライセンス

MIT License。詳細は [LICENSE](./LICENSE) を参照してください。
