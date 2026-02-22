---
title: "git worktree で並行開発してたら、ポート管理が地獄だったので CLI を作った"
tags:
  - Go
  - git
  - CLI
  - 開発環境
  - worktree
private: false
updated_at: ""
id: null
organization_url_name: null
slide: false
ignorePublish: false
---

## "you should use worktrees"

こんなツイートが流れてきた。

https://x.com/_colemurray/status/2025170703448985849

> you should use worktrees
>
> you just have to..
> - npm install in the worktree
> - reinstall the pre-commit hooks
> - copy the env files
> - not use the same ports
>
> or realize this is not the right solution

笑ったし、めちゃくちゃ共感した。そして思った — **「最後の1行以外は、全部自動化できるのでは？」**

---

## ポート管理が地獄になるまで

モノレポで開発していると、フロントエンドは `:3000`、バックエンドは `:8000` で起動するのが当たり前になる。ブランチが1つなら問題ない。

でも git worktree を使い始めると、世界が変わる。

```bash
git worktree add ../myapp-feature-auth feature/auth
git worktree add ../myapp-fix-header fix/header
```

main、feature/auth、fix/header — 3つのブランチを同時に動かしたい。でもフロントエンドを3つ起動すると、`:3000` は1つしか使えない。手動でポートをずらす？ `:3001`、`:3002`... どのポートがどのブランチだったか、30分後にはもう覚えていない。

バックエンドも同じ。環境変数を書き換えて、フロントからバックエンドへの接続先を変えて、ブランチを切り替えるたびに設定をやり直す。

あのツイートの「not use the same ports」が、一番静かに一番痛いやつだ。

**これは仕組みで解決すべき問題だ**、と思った。

---

## portless の存在を知った

portree をほぼ作り終えた後に、Vercel Labs の **[portless](https://github.com/vercel-labs/portless)** の存在を知った。まじで昨日まで知らなかった。

ポート番号を `myapp.localhost:1355` のような名前付き URL に置き換えるツールで、ローカル開発の DX を改善するというアプローチが近い。正直「先にやられてた」という気持ちが一瞬よぎった。

ただ、よく見ると解決している問題のスコープが違う。

portless は「ポートに名前をつける」ツール。すでに起動しているサーバーに対するプロキシであって、サーバーの起動・停止・ライフサイクル管理は範囲外だ。git worktree ごとにサーバーをまとめて管理するという概念もない。

自分が欲しかったのは、**「worktree を追加したら、全サービスが自動で正しいポートで起動して、ブラウザからブランチ名でアクセスできる」** という体験だった。ポートに名前をつけるだけでは足りない。ポートの割り当て、プロセスの起動・停止、サービス間のディスカバリ、全部まとめて面倒を見てほしい。

---

## portree を作った

**[portree](https://github.com/fairy-pitta/portree)** — Git Worktree Server Manager。

名前は port + tree から。Go で書いた。

```bash
# 初期化
portree init

# 全 worktree のサービスを一括起動
portree up --all

# ブラウザで開く
portree open
# → http://main.localhost:3000
```

やりたかったことは3つ。

### 1. ポートの衝突を仕組みで消す

ブランチ名とサービス名から FNV32 ハッシュでポートを決定的に割り当てる。

```
FNV32("main:frontend") % 100 + 3100 → 3100
FNV32("feature/auth:frontend") % 100 + 3100 → 3117
```

同じブランチ・同じサービスなら、何度再起動しても同じポート。手動で覚える必要がない。万が一衝突しても、linear probing で次の空きポートに自動で逃げる。

### 2. サーバーのライフサイクルを一元管理する

`.portree.toml` に1回定義すれば、全 worktree で同じ構成が動く。

```toml
[services.frontend]
command = "pnpm run dev"
dir = "frontend"
port_range = { min = 3100, max = 3199 }
proxy_port = 3000

[services.backend]
command = "python manage.py runserver 0.0.0.0:$PORT"
dir = "backend"
port_range = { min = 8100, max = 8199 }
proxy_port = 8000
```

`portree up --all` で全 worktree の全サービスが起動し、`portree down --all` でまとめて停止。プロセスグループ単位で管理しているので、子プロセスの取りこぼしもない。SIGTERM を送って、タイムアウトしたら SIGKILL。

### 3. ブランチ名でアクセスできる

`portree proxy start` でリバースプロキシを起動すると、`Host` ヘッダのサブドメインでルーティングする。

```
http://main.localhost:3000          → frontend (main)
http://feature-auth.localhost:3000  → frontend (feature/auth)
http://main.localhost:8000          → backend (main)
http://feature-auth.localhost:8000  → backend (feature/auth)
```

`*.localhost` は [RFC 6761](https://tools.ietf.org/html/rfc6761) でブラウザが `127.0.0.1` に解決することが規定されているので、`/etc/hosts` を編集する必要がない。

---

## 作っていて面白かったところ

### ポート割り当ての TOCTOU 問題

「このポート空いてるかチェック → 割り当て → サービスが bind する」の間にタイムラグがある。別のプロセスがその隙間でポートを奪う可能性がある（Time-of-Check-Time-of-Use）。

完全に排除はできないが、ファイルレベルの排他ロック（`flock`）で portree の複数インスタンス間の競合は防いでいる。外部プロセスとの競合が起きた場合は、明確なエラーメッセージで原因がわかるようにした。

### プロセスグループの罠

開発サーバーは子プロセスを spawn することが多い（Next.js の SWC コンパイラなど）。単にメインプロセスを kill しても子が残る。

```go
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
```

`Setpgid: true` でプロセスグループを作り、停止時は `syscall.Kill(-pgid, syscall.SIGTERM)` でグループごと落とす。これで子プロセスの孤立を防いでいる。

### リバースプロキシの WriteTimeout

Go の `http.Server` には `WriteTimeout` を設定するのが定石だが、dev server のプロキシでは意図的に `0`（無制限）にしている。Vite や webpack の HMR は SSE（Server-Sent Events）で接続を張りっぱなしにするので、固定の write deadline を設定するとストリームが切断されてしまう。

```go
srv := &http.Server{
    ReadTimeout:       30 * time.Second,
    ReadHeaderTimeout: 10 * time.Second,
    IdleTimeout:       120 * time.Second,
    // WriteTimeout は意図的に 0: HMR の SSE ストリームを殺さない
}
```

「セキュリティのベストプラクティスに従う」のではなく、「ユースケースを理解して設定を選ぶ」。ローカル開発ツールならではの判断だった。

---

## TUI ダッシュボード

全 worktree の全サービスの状態を一覧で見て、vim ライクなキーバインドで操作できる TUI も作った。[Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss) で実装。

```
╭─ portree dashboard ────────────────────────────────╮
│                                                     │
│  WORKTREE        SERVICE    PORT   STATUS    PID    │
│ ▸ main           frontend   3100   ● running 12345  │
│   main           backend    8100   ● running 12346  │
│   feature/auth   frontend   3117   ○ stopped —      │
│                                                     │
│  [s] start  [x] stop  [r] restart  [q] quit        │
╰─────────────────────────────────────────────────────╯
```

`portree dash` で起動して、`s` で起動、`x` で停止、`o` でブラウザを開く。ターミナルから手を離さずに全ブランチを管理できる。

---

## portless との違い

同じ「ローカル開発の DX 改善」という領域だが、アプローチが異なる。

| | portless | portree |
|---|---|---|
| 思想 | ポートを名前に置き換える | worktree 単位で開発環境を管理する |
| プロセス管理 | なし（プロキシのみ） | 起動・停止・ライフサイクル全体 |
| ポート割当 | ランダム | FNV32 ハッシュで決定論的 |
| 名前付き URL | あり | あり（`branch-name.localhost`） |
| worktree 対応 | なし | コア機能 |
| HTTPS | あり（自動証明書） | 対応中 |
| TUI | なし | あり |
| 言語 | TypeScript | Go（シングルバイナリ） |

portless は「どのプロジェクトでも使える汎用ツール」、portree は「git worktree を使う開発者のための専用ツール」。名前付き URL やHTTPS といった機能は共通して持っている（portree の HTTPS は現在対応中）が、portree はその上にプロセス管理・ポート自動割当・worktree 統合・TUI を載せている。

git worktree で並行開発をしているなら、portree のほうが刺さるはず。worktree を使っていなくても、モノレポで複数サービスを管理するなら十分に便利だと思う。

---

## インストール

```bash
# Homebrew
brew install fairy-pitta/tap/portree

# Go
go install github.com/fairy-pitta/portree@latest
```

GitHub: **[fairy-pitta/portree](https://github.com/fairy-pitta/portree)**

フィードバック・Issue・Star、なんでも歓迎です。
