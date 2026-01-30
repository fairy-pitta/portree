# gws - Git Worktree Server Manager 実装計画

## 概要

git worktree ごとに **複数の dev server** を自動管理する CLI ツール。
worktree 単位でフロントエンド・バックエンドなど複数サービスを起動し、
`branch-name.localhost:<proxy_port>` でリバースプロキシ経由アクセスできる。

- **言語**: Go
- **名前**: `gws`
- **ライセンス**: MIT

## 想定ユースケース

```
my-project/              # git repo (main worktree)
├── frontend/            # Next.js
├── backend/             # Django
└── .gws.toml

../my-project-feature-auth/   # git worktree (feature/auth)
├── frontend/
└── backend/
```

各 worktree で frontend + backend を同時に立てたい。
branch ごとにポートが異なるので衝突しない。
`feature-auth.localhost:3000` でフロント、`feature-auth.localhost:8000` でバックエンド。

---

## ユーザーコマンド

```
gws init                          # .gws.toml を生成
gws up [--all] [--service NAME]   # dev server 起動
gws down [--all] [--service NAME] # dev server 停止
gws ls                            # worktree × service 一覧 + 状態表示
gws dash                          # TUI ダッシュボード
gws proxy start                   # リバースプロキシ起動
gws proxy stop                    # リバースプロキシ停止
gws open [--service NAME]         # ブラウザでアクセス
gws version                       # バージョン表示
```

### コマンド詳細

| コマンド          | 引数/フラグ                                              | 説明                                                      |
| ----------------- | -------------------------------------------------------- | --------------------------------------------------------- |
| `gws init`        | —                                                        | `.gws.toml` をカレントリポジトリのルートに生成            |
| `gws up`          | `--all`: 全 worktree, `--service NAME`: 特定サービスのみ | デフォルトは現在の worktree の全サービスを起動            |
| `gws down`        | 同上                                                     | 起動中のサーバーを停止                                    |
| `gws ls`          | —                                                        | 全 worktree × 全 service の一覧 (ポート、PID、状態)       |
| `gws dash`        | —                                                        | Bubble Tea TUI ダッシュボード                             |
| `gws proxy start` | —                                                        | 全 `proxy_port` でリバースプロキシ起動 (バックグラウンド) |
| `gws proxy stop`  | —                                                        | リバースプロキシ停止                                      |
| `gws open`        | `--service NAME` (default: 最初のサービス)               | ブラウザで `branch.localhost:proxy_port` を開く           |
| `gws version`     | —                                                        | バージョン表示 (ldflags 埋め込み)                         |

---

## 設定ファイル (.gws.toml)

```toml
# gws - Git Worktree Server Manager configuration

# --- サービス定義 ---
# worktree ごとに起動するサービスを定義する。
# 各サービスは独立したコマンド、ポートレンジ、プロキシポートを持つ。

[services.frontend]
command = "pnpm run dev"
dir = "frontend"                       # worktree ルートからの相対パス (省略時はルート)
port_range = { min = 3100, max = 3199 } # このサービス用のポート割り当て範囲
proxy_port = 3000                       # プロキシがlistenするポート

[services.backend]
command = "source .venv/bin/activate && python manage.py runserver 0.0.0.0:$PORT"
dir = "backend"
port_range = { min = 8100, max = 8199 }
proxy_port = 8000

# --- 共通環境変数 ---
[env]
NODE_ENV = "development"

# --- worktree ごとの上書き (任意) ---
# [worktrees.main]
# services.frontend.port = 3100       # ポート固定
#
# [worktrees."feature/auth"]
# services.backend.command = "source .venv/bin/activate && python manage.py runserver --settings=myapp.settings_auth 0.0.0.0:$PORT"
# services.backend.env = { DEBUG = "1" }
```

### 設定の構造体

```go
type Config struct {
    Services  map[string]ServiceConfig `toml:"services"`
    Env       map[string]string        `toml:"env"`
    Worktrees map[string]WTOverride    `toml:"worktrees"`
}

type ServiceConfig struct {
    Command   string    `toml:"command"`
    Dir       string    `toml:"dir"`        // worktree相対パス
    PortRange PortRange `toml:"port_range"`
    ProxyPort int       `toml:"proxy_port"`
}

type PortRange struct {
    Min int `toml:"min"`
    Max int `toml:"max"`
}

type WTOverride struct {
    Services map[string]WTServiceOverride `toml:"services"`
}

type WTServiceOverride struct {
    Command string            `toml:"command,omitempty"`
    Port    int               `toml:"port,omitempty"`  // ポート固定
    Env     map[string]string `toml:"env,omitempty"`
}
```

---

## ディレクトリ構成

```
gws/
├── main.go
├── go.mod
├── Makefile
├── .goreleaser.yaml
├── .github/workflows/
│   ├── ci.yaml
│   └── release.yaml
├── cmd/
│   ├── root.go          # PersistentPreRunE: repo root 検出, config 読込
│   ├── init.go          # gws init
│   ├── up.go            # gws up [--all] [--service]
│   ├── down.go          # gws down [--all] [--service]
│   ├── ls.go            # gws ls
│   ├── dash.go          # gws dash
│   ├── open.go          # gws open [--service]
│   ├── proxy.go         # gws proxy start|stop
│   └── version.go       # gws version
├── internal/
│   ├── config/          # .gws.toml の読込・検証
│   │   └── config.go
│   ├── git/             # worktree 検出
│   │   ├── worktree.go  # ListWorktrees, CurrentWorktree, BranchSlug
│   │   └── repo.go      # FindRepoRoot, CommonDir, MainWorktreeRoot
│   ├── port/            # ポート割り当て
│   │   ├── allocator.go # hash ベース割り当て + linear probe
│   │   └── registry.go  # 永続化・管理
│   ├── process/         # 子プロセスライフサイクル管理
│   │   ├── manager.go   # 複数 Runner の統括
│   │   └── runner.go    # 単一プロセスラッパー
│   ├── proxy/           # リバースプロキシ
│   │   ├── server.go    # HTTP サーバー + ReverseProxy
│   │   └── resolver.go  # slug + proxy_port → 実ポート解決
│   ├── state/           # JSON ファイルベースの状態管理
│   │   └── store.go     # FileStore (flock ベースロック)
│   ├── tui/             # Bubble Tea TUI ダッシュボード
│   │   ├── app.go       # トップレベル Model
│   │   ├── dashboard.go # テーブルモデル
│   │   ├── styles.go    # Lip Gloss スタイル定義
│   │   ├── keys.go      # キーバインド
│   │   └── messages.go  # カスタムメッセージ型
│   └── browser/         # ブラウザ起動ユーティリティ
│       └── open.go
└── docs/
```

---

## 主要な設計判断

### 1. 複数サービスモデル

**1 worktree = N services**。各 service は独立した:

- コマンド (`command`)
- 作業ディレクトリ (`dir`)
- ポートレンジ (`port_range`)
- プロキシポート (`proxy_port`)

を持つ。`gws up` は現在の worktree の全サービスを起動、
`gws up --service frontend` で特定サービスのみ起動できる。

### 2. ポート割り当て

サービスごとに独立したレンジからハッシュベースで割り当てる:

```
port = fnv32(branch_name) % (max - min + 1) + min
```

衝突時は linear probe で次の空きポートを探す。
割り当ては `.gws/state.json` に永続化し、再起動間で安定させる。

worktree ごとの固定ポートは `[worktrees."branch".services.name]` の `port` で上書き可能。

### 3. 環境変数の自動注入

各プロセスに以下の環境変数を自動注入する:

| 変数                 | 例                                                   | 説明                                     |
| -------------------- | ---------------------------------------------------- | ---------------------------------------- |
| `PORT`               | `3117`                                               | そのサービスの割り当てポート             |
| `GWS_BRANCH`         | `feature/auth`                                       | ブランチ名                               |
| `GWS_BRANCH_SLUG`    | `feature-auth`                                       | URL-safe スラッグ                        |
| `GWS_SERVICE`        | `frontend`                                           | サービス名                               |
| `GWS_<SERVICE>_PORT` | `GWS_FRONTEND_PORT=3117`                             | 同一 worktree の各サービスの実ポート     |
| `GWS_<SERVICE>_URL`  | `GWS_BACKEND_URL=http://feature-auth.localhost:8000` | 同一 worktree の各サービスのプロキシ URL |

これにより、frontend から backend への通信設定を環境変数で自動解決できる:

```js
// next.config.js
module.exports = {
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${process.env.GWS_BACKEND_URL}/api/:path*`,
      },
    ];
  },
};
```

### 4. プロセス管理

- `sh -c "command"` で起動 (`dir` に `cd` した上で)
- `Setpgid: true` でプロセスグループ作成
- 停止時: SIGTERM → 10 秒待機 → SIGKILL (プロセスグループ全体)
- stdout/stderr は `.gws/logs/{branch_slug}.{service}.log` に書き出し
- PID は `state.json` に記録

### 5. 状態管理 (state.json)

`.gws/state.json` に以下を保存:

```json
{
  "services": {
    "main": {
      "frontend": {
        "port": 3100,
        "pid": 12345,
        "status": "running",
        "started_at": "2026-01-31T00:00:00Z"
      },
      "backend": {
        "port": 8100,
        "pid": 12346,
        "status": "running",
        "started_at": "2026-01-31T00:00:00Z"
      }
    },
    "feature/auth": {
      "frontend": {
        "port": 3117,
        "pid": 0,
        "status": "stopped",
        "started_at": ""
      }
    }
  },
  "proxy": {
    "pid": 99999,
    "status": "running"
  },
  "port_assignments": {
    "main:frontend": 3100,
    "main:backend": 8100,
    "feature/auth:frontend": 3117,
    "feature/auth:backend": 8104
  }
}
```

ファイルロック: `syscall.Flock` で排他制御。

### 6. リバースプロキシ

`proxy_port` ごとに HTTP サーバーを起動 (`httputil.ReverseProxy` + `Rewrite`):

```
:3000 → frontend 用プロキシ
:8000 → backend 用プロキシ
```

リクエスト処理フロー:

1. `Host` ヘッダーから slug を抽出: `feature-auth.localhost:3000` → `feature-auth`
2. slug → branch 名に逆引き
3. そのプロキシポートに対応するサービスを特定 (3000 → frontend)
4. `state.json` から実ポートを取得 (feature/auth の frontend → 3117)
5. `http://127.0.0.1:3117` にリバースプロキシ

### 7. `*.localhost` の名前解決

RFC 6761 により、最新ブラウザは `*.localhost` を `127.0.0.1` に解決する。
追加の DNS 設定やホストファイル編集は不要。

---

## 外部依存

| パッケージ                           | 用途                          |
| ------------------------------------ | ----------------------------- |
| `github.com/spf13/cobra`             | CLI フレームワーク            |
| `github.com/BurntSushi/toml`         | TOML 設定パース               |
| `github.com/charmbracelet/bubbletea` | TUI フレームワーク            |
| `github.com/charmbracelet/bubbles`   | TUI コンポーネント (table 等) |
| `github.com/charmbracelet/lipgloss`  | TUI スタイリング              |

---

## 実装フェーズ

### Phase 1: 基盤 (git 検出 + config)

| #   | ファイル                    | 内容                                                                                      |
| --- | --------------------------- | ----------------------------------------------------------------------------------------- |
| 1   | `go.mod`, `main.go`         | Go module 初期化 + エントリポイント                                                       |
| 2   | `internal/git/repo.go`      | `FindRepoRoot()`, `CommonDir()`, `MainWorktreeRoot()`                                     |
| 3   | `internal/git/worktree.go`  | `Worktree` 構造体, `ListWorktrees()`, `CurrentWorktree()`, `BranchSlug()`                 |
| 4   | `internal/config/config.go` | `Config` 構造体 (複数 services 対応), `Load()`, `DefaultConfig()`, `Validate()`, `Init()` |
| 5   | `cmd/root.go`               | Cobra ルートコマンド + `PersistentPreRunE` (repo root 検出, config 読込)                  |
| 6   | `cmd/init.go`               | `gws init` — `.gws.toml` スキャフォールド                                                 |
| 7   | `cmd/ls.go` (v1)            | worktree × service 一覧表示 (静的情報のみ)                                                |

### Phase 2: プロセス管理

| #   | ファイル                      | 内容                                                                |
| --- | ----------------------------- | ------------------------------------------------------------------- |
| 8   | `internal/state/store.go`     | `FileStore` — JSON 読み書き + `syscall.Flock`                       |
| 9   | `internal/port/allocator.go`  | `Allocate(branch, service)` — FNV32 ハッシュ + linear probe         |
| 10  | `internal/port/registry.go`   | `Registry` — 割り当て管理・永続化・解放                             |
| 11  | `internal/process/runner.go`  | `Runner` — 単一プロセスの起動/停止/ログ管理                         |
| 12  | `internal/process/manager.go` | `Manager` — 複数 Runner の統括 (Start/Stop/Status/StartAll/StopAll) |
| 13  | `cmd/up.go`                   | `gws up [--all] [--service NAME]`                                   |
| 14  | `cmd/down.go`                 | `gws down [--all] [--service NAME]`                                 |
| 15  | `cmd/ls.go` (v2)              | ステータス・ポート・PID 列追加                                      |

### Phase 3: リバースプロキシ + ブラウザ

| #   | ファイル                     | 内容                                                                  |
| --- | ---------------------------- | --------------------------------------------------------------------- |
| 16  | `internal/proxy/resolver.go` | `Resolver` — slug + proxy_port → 実ポート解決                         |
| 17  | `internal/proxy/server.go`   | `ProxyServer` — HTTP サーバー + ReverseProxy (proxy_port ごとに 1 つ) |
| 18  | `cmd/proxy.go`               | `gws proxy start\|stop`                                               |
| 19  | `internal/browser/open.go`   | OS 検出 + `open` / `xdg-open`                                         |
| 20  | `cmd/open.go`                | `gws open [--service NAME]`                                           |

### Phase 4: TUI ダッシュボード

| #   | ファイル                    | 内容                                               |
| --- | --------------------------- | -------------------------------------------------- |
| 21  | `internal/tui/styles.go`    | Lip Gloss スタイル定義                             |
| 22  | `internal/tui/messages.go`  | カスタムメッセージ型 (TickMsg, StatusUpdateMsg 等) |
| 23  | `internal/tui/keys.go`      | キーバインド定義                                   |
| 24  | `internal/tui/dashboard.go` | テーブルモデル (worktree × service 行)             |
| 25  | `internal/tui/app.go`       | トップレベル Model (Init/Update/View)              |
| 26  | `cmd/dash.go`               | `gws dash`                                         |

### Phase 5: 仕上げ + リリース

| #   | ファイル                         | 内容                                         |
| --- | -------------------------------- | -------------------------------------------- |
| 27  | `cmd/version.go`                 | `gws version` — ldflags でバージョン埋め込み |
| 28  | `Makefile`                       | build / test / lint ターゲット               |
| 29  | `.goreleaser.yaml`               | GoReleaser 設定                              |
| 30  | `.github/workflows/ci.yaml`      | CI (test + lint + build)                     |
| 31  | `.github/workflows/release.yaml` | タグ push 時 GoReleaser でリリース           |
| 32  | `README.md` + `README.ja.md`     | 英語 + 日本語ドキュメント                    |
| 33  | テスト                           | unit test + integration test (網羅的)        |

---

## コミット戦略

Phase/ステップごとに逐一コミットを行う。1つの論理的変更 = 1コミット。

| コミットポイント | メッセージ例                                                 |
| ---------------- | ------------------------------------------------------------ |
| Phase 1-1        | `feat: initialize go module and project structure`           |
| Phase 1-2,3      | `feat: add git repo and worktree detection`                  |
| Phase 1-4        | `feat: add multi-service config loading and validation`      |
| Phase 1-5        | `feat: add cobra root command with repo detection`           |
| Phase 1-6        | `feat: add gws init command`                                 |
| Phase 1-7        | `feat: add gws ls command (static info)`                     |
| Phase 1 テスト   | `test: add unit tests for git and config packages`           |
| Phase 2-8        | `feat: add JSON file-based state store with flock`           |
| Phase 2-9,10     | `feat: add hash-based port allocator and registry`           |
| Phase 2-11,12    | `feat: add process runner and manager`                       |
| Phase 2-13,14    | `feat: add gws up and gws down commands`                     |
| Phase 2-15       | `feat: enhance gws ls with status, port, PID columns`        |
| Phase 2 テスト   | `test: add unit tests for state, port, and process packages` |
| Phase 3-16,17    | `feat: add reverse proxy with subdomain routing`             |
| Phase 3-18       | `feat: add gws proxy start/stop commands`                    |
| Phase 3-19,20    | `feat: add browser open utility and gws open command`        |
| Phase 3 テスト   | `test: add unit tests for proxy and browser packages`        |
| Phase 4          | `feat: add TUI dashboard with bubbletea`                     |
| Phase 4 テスト   | `test: add tests for TUI components`                         |
| Phase 5-27       | `feat: add gws version command with ldflags`                 |
| Phase 5-28,29    | `chore: add Makefile and goreleaser config`                  |
| Phase 5-30,31    | `ci: add GitHub Actions for CI and release`                  |
| Phase 5-32       | `docs: add README.md (EN) and README.ja.md (JA)`             |
| Phase 5-33       | `test: add integration tests`                                |

---

## テスト計画

### 方針

- `go test ./... -race -count=1` で全テスト通過を保証
- テーブルドリブンテストを基本スタイルとする
- 外部依存 (git コマンド等) が必要なテストは `testutil` ヘルパーで一時 repo を作成
- カバレッジ目標: 各パッケージ 80% 以上

### パッケージ別テスト一覧

#### `internal/git/` — `git_test.go`

| テスト名               | 内容                                                                                                   |
| ---------------------- | ------------------------------------------------------------------------------------------------------ |
| `TestFindRepoRoot`     | 通常 repo、サブディレクトリ、worktree 内、非 repo でエラー                                             |
| `TestCommonDir`        | main worktree と追加 worktree で共通 dir が一致                                                        |
| `TestMainWorktreeRoot` | worktree 内から main root を正しく返す                                                                 |
| `TestListWorktrees`    | 0個、1個 (main のみ)、3個の worktree を正しくパース                                                    |
| `TestCurrentWorktree`  | main 内、worktree 内、サブディレクトリ内で正しく検出                                                   |
| `TestBranchSlug`       | `main`→`main`, `feature/auth`→`feature-auth`, `fix/bug-123`→`fix-bug-123`, 先頭/末尾特殊文字、空文字列 |
| `TestParsePorcelain`   | 正常出力、bare repo、detached HEAD、空出力                                                             |

#### `internal/config/` — `config_test.go`

| テスト名                        | 内容                                                                             |
| ------------------------------- | -------------------------------------------------------------------------------- |
| `TestDefaultConfig`             | デフォルト値が正しいこと                                                         |
| `TestLoad`                      | 正常な TOML ファイル読み込み                                                     |
| `TestLoadFileNotFound`          | ファイルなしで適切なエラー                                                       |
| `TestLoadInvalidTOML`           | 壊れた TOML でパースエラー                                                       |
| `TestValidate`                  | 正常設定、空 command、不正 port_range、重複 proxy_port、ポート範囲外の固定ポート |
| `TestValidateProxyPortConflict` | 複数サービスが同じ proxy_port を使う場合エラー                                   |
| `TestInit`                      | ファイル生成、既存ファイルでエラー                                               |
| `TestCommandForBranch`          | グローバルデフォルト、worktree 上書きあり/なし                                   |
| `TestEnvForBranch`              | グローバル env のみ、worktree env マージ、上書き優先順位                         |
| `TestFixedPortForBranch`        | 固定あり、固定なし (0 返却)                                                      |

#### `internal/state/` — `store_test.go`

| テスト名                   | 内容                                              |
| -------------------------- | ------------------------------------------------- |
| `TestNewFileStore`         | ディレクトリ自動作成                              |
| `TestLoadSave`             | 空状態の初回ロード、保存→再ロードで一致           |
| `TestConcurrentAccess`     | 複数 goroutine からの同時読み書きでデータ破損なし |
| `TestCorruptedFile`        | 壊れた JSON でエラーハンドリング                  |
| `TestSetGetServiceState`   | サービス状態の設定・取得                          |
| `TestSetGetPortAssignment` | ポート割り当ての設定・取得                        |
| `TestProxyState`           | プロキシ PID・状態の保存・読み取り                |

#### `internal/port/` — `allocator_test.go`, `registry_test.go`

| テスト名                        | 内容                                             |
| ------------------------------- | ------------------------------------------------ |
| `TestAllocate`                  | 同じ branch+service で同じポートが返る (冪等性)  |
| `TestAllocateDifferentBranches` | 異なるブランチで異なるポートが割り当てられる     |
| `TestAllocateCollision`         | ハッシュ衝突時に linear probe で次のポートを返す |
| `TestAllocateRangeFull`         | レンジが埋まった場合のエラー                     |
| `TestAllocateFixedPort`         | 固定ポート指定時にそれが返る                     |
| `TestAllocateFixedPortConflict` | 固定ポートが既に使用中の場合のエラー             |
| `TestRelease`                   | 解放後に再割り当て可能                           |
| `TestRegistryPersistence`       | 割り当て→保存→再ロードで一致                     |
| `TestRegistryGetPort`           | 存在するキー、存在しないキー                     |

#### `internal/process/` — `runner_test.go`, `manager_test.go`

| テスト名                         | 内容                                        |
| -------------------------------- | ------------------------------------------- |
| `TestRunnerStart`                | `echo hello` 等の簡単なコマンドが起動・完了 |
| `TestRunnerStartAlreadyRunning`  | 二重起動でエラー                            |
| `TestRunnerStop`                 | SIGTERM で正常停止                          |
| `TestRunnerStopTimeout`          | SIGTERM 無視するプロセスに SIGKILL          |
| `TestRunnerStopNotRunning`       | 未起動で停止してもエラーなし                |
| `TestRunnerIsRunning`            | 起動中 true、停止後 false、PID 死亡で false |
| `TestRunnerLogOutput`            | stdout/stderr がログファイルに書き出される  |
| `TestRunnerEnvironment`          | PORT, GWS\_\* 環境変数が注入される          |
| `TestRunnerWorkingDir`           | dir 設定が正しく適用される                  |
| `TestManagerStartAll`            | 全サービス起動                              |
| `TestManagerStopAll`             | 全サービス停止                              |
| `TestManagerStartService`        | 特定サービスのみ起動                        |
| `TestManagerStatusAll`           | 全状態取得                                  |
| `TestManagerStaleProcessCleanup` | 死んだ PID の自動クリーンアップ             |

#### `internal/proxy/` — `resolver_test.go`, `server_test.go`

| テスト名                       | 内容                                                                                            |
| ------------------------------ | ----------------------------------------------------------------------------------------------- |
| `TestResolverResolve`          | 正しい slug+proxy_port → 実ポート                                                               |
| `TestResolverUnknownSlug`      | 未知の slug でエラー                                                                            |
| `TestResolverUnknownProxyPort` | 未知の proxy_port でエラー                                                                      |
| `TestParseSlugFromHost`        | `feature-auth.localhost:3000`→`feature-auth`, `localhost:3000`→`""`, `a.b.localhost:3000`→`a.b` |
| `TestProxyServerRouting`       | httptest.Server でプロキシ経由リクエストが正しい backend に到達                                 |
| `TestProxyServer404`           | 未知 slug で 404 レスポンス                                                                     |
| `TestProxyServerHeaders`       | X-Forwarded-For, X-Forwarded-Host が付与される                                                  |
| `TestProxyServerWebSocket`     | WebSocket Upgrade ヘッダーのパススルー                                                          |

#### `internal/browser/` — `open_test.go`

| テスト名                | 内容                                    |
| ----------------------- | --------------------------------------- |
| `TestBuildURL`          | slug + proxy_port から正しい URL を構築 |
| `TestDetectOpenCommand` | darwin→`open`, linux→`xdg-open`         |

#### `internal/tui/` — `app_test.go`, `dashboard_test.go`

| テスト名                      | 内容                                            |
| ----------------------------- | ----------------------------------------------- |
| `TestDashboardInit`           | 初期状態が正しい                                |
| `TestDashboardKeyBindings`    | 各キーで正しいメッセージが発行される            |
| `TestDashboardView`           | View 出力にテーブルヘッダー、行データが含まれる |
| `TestDashboardStatusUpdate`   | StatusUpdateMsg でテーブルが更新される          |
| `TestDashboardCursorMovement` | j/k/↑/↓ でカーソルが正しく移動                  |

#### `cmd/` — `cmd_test.go` (統合テスト)

| テスト名                       | 内容                                                |
| ------------------------------ | --------------------------------------------------- |
| `TestInitCommand`              | 一時 repo で `gws init` → `.gws.toml` 生成確認      |
| `TestInitCommandAlreadyExists` | 既存ファイルでエラー                                |
| `TestLsCommand`                | worktree 一覧の出力フォーマット確認                 |
| `TestUpDownCommand`            | `gws up` → プロセス起動確認 → `gws down` → 停止確認 |
| `TestUpAllCommand`             | `--all` で複数 worktree の全サービス起動            |
| `TestUpServiceFilter`          | `--service frontend` でフロントのみ起動             |
| `TestVersionCommand`           | バージョン文字列出力                                |
| `TestRootNoGitRepo`            | git repo 外で適切なエラー                           |
| `TestRootNoConfig`             | config なしで適切なエラー (init 以外)               |

---

## README 計画

### `README.md` (英語)

- Overview / What is gws
- Features (multi-service, auto port, subdomain proxy, TUI, env injection)
- Quick Start (install → init → configure → up → proxy → open)
- Configuration reference (.gws.toml full spec)
- Commands reference (全コマンドの usage)
- Environment Variables (自動注入変数一覧)
- How It Works (architecture diagram: worktree → services → ports → proxy)
- FAQ (\*.localhost, port conflicts, etc.)
- Contributing
- License (MIT)

### `README.ja.md` (日本語)

- 上記と同一構成の日本語版
- 冒頭に "English version: [README.md](./README.md)" リンク

---

## 動作例

### 初期化

```bash
$ cd my-project
$ gws init
Created .gws.toml in /home/user/my-project
Edit the file to configure your services.
```

### 起動

```bash
# 現在の worktree の全サービスを起動
$ gws up
Starting frontend (port 3100) in /home/user/my-project/frontend ...
Starting backend (port 8100) in /home/user/my-project/backend ...
✓ 2 services started for main

# 全 worktree の全サービスを起動
$ gws up --all
Starting frontend (port 3100) for main ...
Starting backend (port 8100) for main ...
Starting frontend (port 3117) for feature/auth ...
Starting backend (port 8104) for feature/auth ...
✓ 4 services started

# 特定サービスのみ
$ gws up --service backend
Starting backend (port 8100) in /home/user/my-project/backend ...
✓ 1 service started for main
```

### 一覧表示

```bash
$ gws ls
WORKTREE        SERVICE    PORT   STATUS    PID
main            frontend   3100   running   12345
main            backend    8100   running   12346
feature/auth    frontend   3117   stopped   —
feature/auth    backend    8104   stopped   —
```

### プロキシ

```bash
$ gws proxy start
Proxy started:
  :3000 → frontend services
  :8000 → backend services

Access:
  http://main.localhost:3000         → frontend (main)
  http://main.localhost:8000         → backend (main)
  http://feature-auth.localhost:3000 → frontend (feature/auth)
  http://feature-auth.localhost:8000 → backend (feature/auth)

$ gws proxy stop
Proxy stopped.
```

### ブラウザを開く

```bash
$ gws open
Opening http://feature-auth.localhost:3000 ...

$ gws open --service backend
Opening http://feature-auth.localhost:8000 ...
```

### 停止

```bash
$ gws down
Stopping frontend (pid 12345) ...
Stopping backend (pid 12346) ...
✓ 2 services stopped for main

$ gws down --all
Stopping all services ...
✓ 4 services stopped
```

---

## TUI ダッシュボード詳細

```
╭─ gws dashboard ──────────────────────────────────────────────╮
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

**キー操作**:

- `s` — カーソル位置のサービスを起動
- `x` — カーソル位置のサービスを停止
- `r` — カーソル位置のサービスを再起動
- `o` — カーソル位置のサービスをブラウザで開く
- `a` — 全サービス起動
- `X` — 全サービス停止
- `p` — プロキシの起動/停止をトグル
- `l` — ログ表示 (tail -f 相当)
- `j/k` または `↑/↓` — カーソル移動
- `q` — 終了

**自動更新**: 2 秒ごとにプロセス状態をポーリング (PID の生存確認)。

---

## 環境変数注入の詳細

`gws up` でプロセスを起動する際、以下の環境変数をマージする:

1. **OS 環境変数** (親プロセスから継承)
2. **`[env]`** (グローバル設定)
3. **`[worktrees."branch".services.name.env]`** (worktree × service 上書き)
4. **GWS 自動注入変数**:

```bash
# 基本情報
PORT=3117
GWS_BRANCH=feature/auth
GWS_BRANCH_SLUG=feature-auth
GWS_SERVICE=frontend

# 同一 worktree の各サービスの実ポート
GWS_FRONTEND_PORT=3117
GWS_BACKEND_PORT=8104

# 同一 worktree の各サービスのプロキシ URL
GWS_FRONTEND_URL=http://feature-auth.localhost:3000
GWS_BACKEND_URL=http://feature-auth.localhost:8000
```

優先順位: 4 > 3 > 2 > 1 (後の設定が優先)

---

## プロキシ詳細設計

### 起動モデル

`gws proxy start` は **別プロセス** としてプロキシを起動し、PID を `state.json` に記録する。

各 `proxy_port` (例: 3000, 8000) に対して goroutine で HTTP サーバーを起動:

```go
for serviceName, svc := range config.Services {
    go startProxyListener(svc.ProxyPort, serviceName)
}
```

### リクエストルーティング

```
Request: GET http://feature-auth.localhost:3000/api/users

1. Host ヘッダーパース: "feature-auth.localhost:3000"
   → slug = "feature-auth"
   → port = 3000

2. proxy_port → service 逆引き: 3000 → "frontend"

3. slug → branch 逆引き:
   state.json の全ブランチの slug を検索
   "feature-auth" → "feature/auth"

4. state.json から実ポート取得:
   services["feature/auth"]["frontend"].port → 3117

5. リバースプロキシ: → http://127.0.0.1:3117/api/users
```

### slug が見つからない場合

404 レスポンスを返す:

```
404 Not Found
gws: no worktree found for slug "unknown-branch"
Available: main, feature-auth
```

### WebSocket 対応

`httputil.ReverseProxy` の `Rewrite` + `ModifyResponse` で対応。
WebSocket の `Upgrade` ヘッダーはそのままパススルーされる。

---

## ファイルロックと同時実行安全性

複数の `gws` コマンドが同時実行される可能性がある (例: 2 つのターミナルで `gws up`)。

`state.json` の読み書きは `syscall.Flock` で排他ロック:

```go
func (s *FileStore) WithLock(fn func() error) error {
    f, _ := os.OpenFile(s.lockPath, os.O_CREATE|os.O_RDWR, 0644)
    defer f.Close()
    syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
    defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
    return fn()
}
```

---

## エラーハンドリング

| 状況               | 対応                                                          |
| ------------------ | ------------------------------------------------------------- |
| `.gws.toml` がない | `gws init` を案内するエラーメッセージ                         |
| git repo でない    | 明確なエラーメッセージ                                        |
| ポートが既に使用中 | 次の空きポートで再試行、ユーザーに通知                        |
| プロセスが起動失敗 | エラーログ表示、他のサービスは続行                            |
| 孤児プロセス検出   | `gws up` 時に古い PID の生存確認、死んでいれば state クリーン |
| state.json 破損    | バックアップから復元、または初期化                            |

---

## 検証方法

1. `gws init` で `.gws.toml` が生成されることを確認
2. git worktree を 3 つ作成し `gws ls` で全て表示されることを確認
3. `gws up` で現在の worktree の frontend + backend が起動し、異なるポートで動作
4. `gws up --all` で全 worktree の全サービスが起動
5. 各プロセスに `PORT`, `GWS_*` 環境変数が注入されていることを確認
6. `gws proxy start` 後、`curl http://branch-name.localhost:3000` でフロント、`:8000` でバックエンドにプロキシ経由アクセス
7. frontend プロセスから `GWS_BACKEND_URL` を使ってバックエンドに通信できることを確認
8. `gws dash` で TUI 表示、キー操作で start/stop/restart が機能
9. `gws down --all` で全プロセスがクリーンに終了、孤児プロセスなし
10. `go test ./... -race` で全テスト通過
