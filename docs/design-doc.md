# go-file-sync 项目设计文档

> 版本：v0.1.0  
> 状态：草稿  
> 最后更新 2026-06-04

---

## 目录

1. [项目概述](#1-项目概述)
2. [核心功能](#2-核心功能)
3. [架构设计](#3-架构设计)
4. [并发模型](#4-并发模型)
5. [模块划分](#5-模块划分)
6. [配置系统](#6-配置系统)
7. [同步引擎](#7-同步引擎)
8. [日志系统](#8-日志系统)
9. [扩展功能（头脑风暴）](#9-扩展功能头脑风暴)
10. [技术选型](#10-技术选型)
11. [项目结构](#11-项目结构)
12. [开发路线](#12-开发路线)

---

## 1. 项目概述

### 1.1 目标

go-file-sync 是一个轻量级的文件目录自动同步工具，用于监视指定文件或目录的变更，并自动同步到目标位置。适用于开发环境热更新、配置文件分发、静态资源部署等场景。
### 1.2 核心能力

- **文件系统监听**：监听文件目录的创建、修改、删除、重命名事件
- **多源同步**：一个源可同步到多个目标位置
- **并发检测**：多 goroutine 并发监听多个目录
- **灵活配置**：YAML 通用配置 + conf 文件分层配置，支持继承
- **日志可配**：控制台输出/文件输出/两者同时
- **增量同步**：首次全量，后续仅同步差异
- **优雅关闭**：捕获退出信号，等待执行中的同步完成后退出

---

## 2. 核心功能

| 功能 | 优先级 | 描述 |
|------|--------|------|
| 并发监听 | P0 | 每个任务独立 goroutine 并发监听源目录 |
| 同步引擎 | P0 | 将源文件变更同步到目标路径 |
| 配置管理 | P0 | YAML 主配置 + conf 嵌套配置 |
| 日志系统 | P0 | 支持控制台和文件两种输出方式 |
| 目录递归 | P0 | 递归监听子目录变化 |
| 首次全量同步 | P0 | 启动时执行一次全量同步，使用 goroutine 池加速 |
| 增量同步 | P1 | 监听事件触发增量同步 |
| 忽略规则 | P1 | 基于 glob 模式的忽略列表 |
| 变化缓冲 | P1 | 防抖/去重，同一文件频繁变更合并为一个事件 |
| 同步队列 | P1 | channel + worker pool 并发处理同步任务 |
| 工作者 | P1 | 可配置数量的 worker goroutine 执行同步操作 |
| 优雅关闭 | P1 | 捕获退出信号，等待正在执行的同步完成后退出 |
| 文件备份 | P2 | 同步前自动备份目标文件 |
| 差异报告 | P2 | 同步前后差异对比输出 |
| 安全校验 | P2 | 同步前校验文件哈希 |
| 同步状态 | P2 | 实时查看同步进度和状态 |
| 协程监控 | P2 | 监控运行中的 goroutine 状态，异常时自动恢复 |

---

## 3. 架构设计

### 3.1 三层架构

```
┌─────────────────────────────────────────────┐
│                 CLI Layer                    │
│     (root.go - 命令行入口、参数解析)          │
├─────────────────────────────────────────────┤
│              Engine Layer                    │
│ ┌──────────┷┬──────────┷┬──────────┐        │
│ │ Watcher  │ │ Syncer   │ │ Logger   │        │
│ │Pool      │ │Pool      │ │(线程安全) │        │
│ │(N goroutine)│(M goroutine)│         │        │
│ └────┬─────┴┬────┬─────┴┬────┬─────┘        │
│      │      │    │     │   │    │           │
│ ┌────┴─────┬┴───┴────┬───┴────┬────────┐    │
│ │Shutdown  │ │Event  │ │Context│       │    │
│ │Signal    │ │Chan   │ │Cancel │       │    │
│ └──────────┴┴────────┴┴────────┴────────┘    │
├─────────────────────────────────────────────┤
│             Config Layer                     │
│ ┌──────────┷┬──────────┷┬──────────┐        │
│ │  YAML     │   Conf    │  Default │        │
│ │ Config    │  Config   │ Values   │        │
│ └──────────┴┴───────────┴┴─────────┘        │
└─────────────────────────────────────────────┘
```

### 3.2 数据流（并发版）

```
启动
  │
  ▼
解析 YAML 通用配置 ——加载/合并 conf 配置树
  │
  ▼
初始化日志系统（线程安全写入器）
  │
  ▼
创建 root Context ——带取消信号
  │
  ├──创建 N 个 Watcher goroutine（每个 task 一个）
  ├──创建 M 个 Syncer Worker goroutine（pool 模式）
  ├──等待 Signal（SIGINT/SIGTERM）
  │
  ▼
首次全量同步（goroutine 池并行复制文件）
  │
  ▼
每个 Watcher goroutine 独立监听 ——产生事件 ——写入 Event Chan
                                                          │
  ┌───────────────────────────────────────────────────────┘
  │
  ▼
Debouncer goroutine（每个 task 一个）
  │
  ▼
Filter goroutine
  │
  ▼
Event Chan ——Syncer Worker Pool (M 个 goroutine)
                                                      │
  对同一文件的多个操作按序执行────────────────────────┘
  │
  └────── 收到退出信号——cancel() ——等待所有 goroutine 退出——关闭日志
```

---

## 4. 并发模型

### 4.1 概述

采用 **Goroutine + Channel + Context** 的经典 CSP 并发模型，核心原则：

| 原则 | 说明 |
|------|------|
| **不要通过共享内存来通信，要通过通信来共享内存** | 所有任务交互走 channel |
| **谁创建，谁负责退出** | 每个 goroutine 通过 context cancel 通知另一个 goroutine 退出 |
| **池化资源** | Watcher 和 Syncer 都采用 pool 模式，数量可配置 |
| **串行按文件** | 同一文件的多个操作（create + write）由同一个 worker 串行处理 |

### 4.2 Goroutine 分布

```
main goroutine
  │
  ├── Signal goroutine（监听 OS 信号）
  │
  ├── Task 1 —— Watcher goroutine (q1)
  │             ├── Debouncer goroutine (q1)
  │             │
  │             └── Event Chan ──┐
  │                               │
  ├── Task 2 —— Watcher goroutine (q2)    ├── Syncer Worker Pool
  │             ├── Debouncer goroutine   │  (M 个 goroutine)
  │             │                        │
  │             └── Event Chan ──────────┘
  │
  ├── Task N —— Watcher goroutine (qN)
  │             └── Debouncer goroutine
  │
  └── Graceful shutdown coordinator
```

**goroutine 数量估算**：
- Watcher goroutines: `N`（N = conf 任务数）
- Debouncer goroutines: `N`（每个 watcher 配一个）
- Syncer workers: `M`（默认= CPU 核心数，YAML 可配）
- 其他：2~3（signal handler、coordinator 等）
- **总计**: `2N + M + 3`

### 4.3 线程安全设计

| 组件 | 线程安全策略 |
|------|-------------|
| Logger | `sync.Mutex` 保护写入，每个日志行原子写入 |
| Config | 启动时加载完只读，全局共享无锁访问 |
| Counter/Stats | `sync/atomic` 原子操作 |
| Event Chan | 无锁 channel 通信 |
| 共享状态 | 封装在 struct 内部，通过方法 + mutex 访问 |
| 文件操作 | 同一文件通过 `fileKey` hash 路由到同一个 worker |

### 4.4 优雅关闭流程

```
收到信号（SIGINT/SIGTERM）
  │
  ▼
调用 cancel() ——所有 Watcher/Debouncer goroutine 退出
  │
  ▼
关闭 Event Chan ——Syncer Worker 处理完队列中剩余事件后退出
  │
  ▼
等待 WaitGroup ——所有 goroutine 确认退出
  │
  ▼
关闭日志文件 ——程序退出
```

**超时保护**：如果等待超过 `shutdown_timeout`（默认 30 秒），强制退出。
### 4.5 错误恢复

- **Watcher 崩溃**：Task 管理器检测到 watcher goroutine 异常退出后自动重启（最多重启 3 次）
- **Worker 崩溃**：Worker Pool 自动启动新的 worker 替换
- **文件操作失败**：记录错误日志，不阻塞后续事件处理

---

## 5. 模块划分

### 5.1 Config 模块 - `pkg/config/`

- **yaml.go**: 解析 `sync.yaml` 通用配置（日志级别、模式、全局忽略列表、workers 数量等）
- **conf.go**: 解析 `*.conf` 任务配置（源路径、目标路径、文件包名、排除模式、继承关系）
- **merge.go**: 合并继承链的配置，最终生成完整的任务配置
- **types.go**: 配置相关的数据结构定义

### 5.2 Watcher 模块 - `pkg/watcher/`

- **watcher.go**: 封装 fsnotify，每个 task 启动一个 watcher goroutine
- **debouncer.go**: 防抖 goroutine，同一文件多次变更合并为一个事件后写入 chan
- **filter.go**: 事件过滤器，基于包含/排除规则决定是否处理

### 5.3 Syncer 模块 - `pkg/syncer/`

- **syncer.go**: 核心同步逻辑，处理文件复制、删除、移动操作
- **pool.go**: Worker Pool 管理，启动 M 个 worker goroutine 从 event chan 消费
- **backup.go**: 同步前备份功能
- **hash.go**: 文件哈希校验，避免无效同步

### 5.4 Logger 模块 - `pkg/logger/`

- **logger.go**: 日志接口定义及实现（线程安全）
- **console.go**: 控制台输出日志（带颜色）
- **file.go**: 文件输出日志（带轮转）
- **level.go**: 日志级别定义

### 5.5 CLI 模块 - `cmd/`

- **root.go**: cobra 根命令
- **run.go**: `run` 子命令（启动同步）
- **status.go**: `status` 子命令（查看同步状态）
- **check.go**: `check` 子命令（测试配置有效性）

---

## 6. 配置系统

### 6.1 YAML 通用配置（`sync.yaml`）

放置在项目用户根目录，存放通用设置：

```yaml
# sync.yaml - 通用配置
log:
  level: info                # debug | info | warn | error
  output: console            # console | file | both
  file:                      # output=file/both 时生效
    path: ./logs/sync.log
    max_size: 100            # MB
    max_backups: 7
    compress: true

watch:
  debounce: 500              # 防抖等待时间（毫秒）
  interval: 100              # 轮询间隔（毫秒）
  workers: 0                 # watcher goroutine 数，0 = 每个 task 一个

sync:
  mode: incremental          # full | incremental
  workers: 4                 # syncer worker goroutine 数，默认 CPU 核心数
  backup: true               # 同步前是否备份
  verify: true               # 是否校验文件哈希
  exclude:                   # 全局忽略规则
    - "*.tmp"
    - "*.log"
    - ".git/"
    - "node_modules/"
    - ".DS_Store"
    - "*.swp"
  include: []                # 全局包含规则

shutdown:
  timeout: 30                # 优雅关闭超时时间（秒）

conf:
  dir: ./conf                # conf 文件目录
  entry: sync.conf           # 入口 conf 文件
```

### 6.2 Conf 任务配置（`*.conf`）

每个 conf 文件定义一个同步任务，**支持嵌套继承**：

```conf
# sync.conf - 入口配置
# 继承链：sync.conf <- dev.conf <- specific.conf
# 子文件继承父文件所有属性，同名属性覆盖

[task]
name = "开发环境同步"
source = "D:/Workspaces/my-project/src"
target = "D:/Deploy/test-server"
mode = "sync"                # sync | mirror
exclude = ["*.bak"]          # 追加到继承的 exclude 列表
include = []                 # 追加
delete_orphans = false       # 是否删除目标端多余文件

[watch]
recursive = true
events = ["create", "write", "remove", "rename"]
debounce = 500               # 可覆盖全局 debounce 设置（毫秒）

[inherit]
# 继承另一个 conf 文件（路径基于 conf.dir 目录）
file = "dev.conf"            # 可选，不指定则不继承
```

**继承规则**：

```
conf A (父)               conf B (子)
┌─────────────────┐      ┌─────────────────┐
│name = "父级"    │      │   name = "子级"  │   <- 覆盖
│source = "/src"  │      │  source = "/src2"│   <- 覆盖
│exclude = [".a"] │ ───>  │exclude = [".b"] │   <- 追加到父列表 -> [".a", ".b"]
│target = "/dst"  │       │  target = "/dst" │   <- 继承
│mode = "sync"    │       │  mode = "sync"  │   <- 继承
└─────────────────┘      └─────────────────┘
```

**继承链深度限制**：最多 10 层，防止循环继承导致栈溢出。
**循环检测**：继承链中检测到循环引用时报错退出。

### 6.3 配置合并优先级

```
命令行参数（最高）
    │
    ▼
Conf 任务配置 (子覆盖父，列表追加)
    │
    ▼
YAML 通用配置
    │
    ▼
内置默认值（最低）
```

---

## 7. 同步引擎

### 7.1 同步模式

| 模式 | 描述 | 行为 |
|------|------|------|
| `sync` | 单向同步 | 源→目标，目标新增/修改不删除目标多余文件 |
| `mirror` | 镜像同步 | 源→目标，完全镜像，目标多余文件会被删除 |

### 7.2 事件处理（并发版）

```
文件系统事件
    │
    ▼
┌──────────────────────┐
│ Watcher goroutine    │ <- 每个 task 一个，独立监听
│监听源目录变化        │
└────────┬─────────────┘
         │
         ▼
┌──────────────────────┐
│ Debouncer goroutine  │ <- 每个 task 一个，接收 watcher 事件
│缓冲 + 去重          │ <- 同一文件 500ms 内多次事件合并
│防抖后输出到 chan    │
└────────┬─────────────┘
         │
         ▼
┌──────────────────────┐
│ Filter（同步调用）    │ <- 匹配 exclude/include 规则
│跳过忽略文件          │
└────────┬─────────────┘
         │
         ▼
┌──────────────────────┐
│ Event Chan           │ <- 每个 task 的过滤后事件汇集到这里
│(带缓冲 channel)      │
└────────┬─────────────┘
         │
         ▼
┌──────────────────────────────────────────────┐
│ Syncer Worker Pool (M 个 goroutine)            │
│                                              │
│ ┌────────┷┬────────┷┬────────┐              │
│ │Worker 1 │ │Worker 2│ │Worker M│             │
│ │文件 A)  │ │文件 B) │ │文件 C) │              │
│ └───┬─────┴┬────┬─────┴┬────┬─────┘              │
│     │      │    │     │   │    │                │
│同一文件 hash 路由到同一 worker（保证按序执行）   │
└──────────────────────────────────────────────┘
         │
         ▼
┌──────────────────────┐
│  执行同步操作        │
│  - create -> 复制     │
│  - write -> 覆盖      │
│  - remove -> 删除     │
│  - rename -> 重命名   │
│  - chmod -> 改权限    │
└──────────────────────┘
```

### 7.3 文件 Hash 路由

为保证同一文件的操作按序执行，使用 **一致性 hash 路由**：

```go
// 根据文件相对路径 hash 选择 worker
workerID := hash(fileRelPath) % len(workerPool)
```

这样同一个文件的所有事件（create、write、remove）都会被同一个 worker 串行处理，避免并发写同一目标文件的冲突。

### 7.4 首次同步

启动时检测目标目录是否为空，为空则执行一次全量同步（full sync），将源目录完整复制到目标目录。

**首次同步使用 goroutine 池加速**：

```go
// 使用与 syncer workers 相同数量的 goroutine 并行复制
sem := make(chan struct{}, syncWorkers)
for _, file := range fileList {
    sem <- struct{}{}
    go func(f string) {
        defer func() { <-sem }()
        copyFile(f)
    }(file)
}
// 等待所有 goroutine 完成
for i := 0; i < syncWorkers; i++ {
    sem <- struct{}{}
}
```

### 7.5 文件过滤

支持两种规则：
- **exclude**（排除）: 匹配的文件不参与同步
- **include**（包含）: 在白名单中的文件才参与同步（优先级高于 exclude）

规则支持 `filepath.Match` 风格的 glob 模式。

---

## 8. 日志系统

### 8.1 日志级别

| 级别 | 数值 | 说明 |
|------|------|------|
| DEBUG | 0 | 调试信息，开发使用 |
| INFO | 1 | 一般信息（默认） |
| WARN | 2 | 警告信息 |
| ERROR | 3 | 错误信息（不影响运行） |

### 8.2 输出模式

| 模式 | 输出目标 |
|------|----------|
| `console` | 仅标准输出，带颜色 |
| `file` | 仅写入日志文件 |
| `both` | 同时输出到控制台和文件 |

### 8.3 线程安全

日志写入使用 `sync.Mutex` 保证原子性，多个 goroutine 同时写入不会产生交错行：

```go
type FileLogger struct {
    mu     sync.Mutex
    file   *os.File
    writer *bufio.Writer
}

func (l *FileLogger) Write(entry LogEntry) {
    l.mu.Lock()
    defer l.mu.Unlock()
    // 写入一条完整的日志行
    fmt.Fprintf(l.writer, format, entry.Time, entry.Level, entry.Module, entry.Message)
}
```

### 8.4 日志格式

```
2026-06-04 10:30:00.123 [INFO]  [syncer] 文件已同步 /src/main.go -> /dst/main.go (2.3KB)
2026-06-04 10:30:00.456 [WARN]  [watcher] 文件访问被拒绝 /src/.cache (权限不足)
2026-06-04 10:30:01.789 [ERROR] [syncer] 同步失败：/src/config.yaml -> /dst/config.yaml (磁盘空间不足)
```

格式：`时间 [级别] [模块] 消息`

---

## 9. 扩展功能（头脑风暴）

### 9.1 计划内扩展

| 功能 | 优先级 | 描述 |
|------|--------|------|
| **远程同步** | P3 | 支持通过 SSH/SCP/RSYNC 同步到远程服务器。配置中 target 支持 `ssh://` 协议 |
| **Web UI** | P3 | 提供简单的 Web 仪表盘，查看同步状态、启停任务 |
| **插件系统** | P3 | 支持插件钩子：同步前/后执行自定义脚本 |

### 9.2 创意扩展

| 功能 | 描述 | 适用场景 |
|------|------|----------|
| **文件版本快照** | 每次同步前对目标文件做快照，支持回滚到任意版本 | 配置文件管理 |
| **差异预览模式** | `--dry-run` 参数，预览将要执行的变更而不实际执行 | 调试配置 |
| **触发式外部命令** | 同步完成后执行外部命令（如重启 Nginx、重启服务） | 热更新部署 |
| **加密同步** | 源文件加密后传输，目标端解密 | 传输敏感文件 |
| **跨平台文件属性** | 同步文件权限、所有者、时间戳等元信息 | Linux 服务器部署 |
| **多源聚合** | 多个源目录同步到同一个目标目录 | 微服务统一部署 |
| **热加载配置** | 配置文件变更后自动重新加载，无需重启 | 长时间运行场景 |
| **定时同步（cron）** | 定时执行同步，而非实时监听 | 批量数据处理 |
| **同步历史** | 记录每次同步的文件列表和变更 | 审计追踪 |
| **网络限速** | 限制同步带宽，避免影响其他网络服务 | 生产环境同步 |
| **冲突检测** | 目标文件在同步前已被修改时发出告警 | 双向同步场景 |
| **邮件/通知** | 同步失败时发送通知（邮件、Webhook、飞书、钉钉） | 运维监控 |

### 9.3 并发相关扩展思路

| 功能 | 描述 |
|------|------|
| **动态调整 Worker** | 运行时根据负载动态调整 syncer worker 数量 |
| **协程监控仪表盘** | 实时展示所有 goroutine 运行状态、channel 积压 |
| **分布式同步** | 多节点通过消息队列协同，每个节点作为 worker |
| **任务优先级** | 不同 conf 任务设置优先级，高优任务分配更多 goroutine |

---

## 10. 技术选型

| 组件 | 选型 | 理由 |
|------|------|------|
| 语言 | Go 1.26 | 编译为单一二进制，跨平台，并发原生支持 |
| 文件监听 | [fsnotify](https://github.com/fsnotify/fsnotify) | Go 生态标准文件监听库 |
| CLI 框架 | [cobra](https://github.com/spf13/cobra) + [viper](https://github.com/spf13/viper) | 标准 Go CLI 工具集 |
| YAML 解析 | [gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) | 官方推荐 YAML 库 |
| TOML/INI 配置 | 自解析（`*.conf` 格式自定义） | 轻量，无需额外依赖 |
| 日志 | slog（标准库） | Go 1.21+ 内置结构化日志，线程安全 |
| 哈希校验 | crypto/sha256（标准库） | 无需额外依赖 |
| 并发控制 | context + sync 标准库 | Go 原生并发原语 |
| 测试 | testing（标准库）+ [testify](https://github.com/stretchr/testify) | 断言库简化测试编写 |

---

## 11. 项目结构

```
go-file-sync/
├── cmd/                          # 命令行入口
│   └── root.go                   # cobra 根命令
├── pkg/
│   ├── config/                   # 配置管理
│   │   ├── types.go              # 数据结构定义
│   │   ├── yaml.go               # YAML 配置解析
│   │   ├── conf.go               # Conf 配置解析
│   │   └── merge.go              # 配置合并逻辑
│   ├── watcher/                  # 文件监听（并发）
│   │   ├── watcher.go            # 监听 goroutine 封装
│   │   ├── debouncer.go          # 防抖 goroutine
│   │   └── filter.go             # 事件过滤器
│   ├── syncer/                   # 同步引擎（并发）
│   │   ├── syncer.go             # 核心同步逻辑
│   │   ├── pool.go               # Worker Pool 管理
│   │   ├── backup.go             # 备份功能
│   │   └── hash.go               # 哈希校验
│   ├── logger/                   # 日志系统（线程安全）
│   │   ├── logger.go             # 日志接口
│   │   ├── console.go            # 控制台输出
│   │   ├── file.go               # 文件输出
│   │   └── level.go              # 日志级别
│   └── core/                     # 核心编排
│       ├── app.go                # 应用生命周期管理
│       └── shutdown.go           # 优雅关闭
├── conf/                         # 配置文件目录
│   ├── sync.conf                 # 入口配置
│   └── examples/                 # 示例配置
│       ├── dev.conf
│       └── prod.conf
├── sync.yaml                     # 通用配置（用户可修改）
├── main.go                       # 程序入口
├── go.mod
├── go.sum
├── Makefile                      # 构建脚本
├── README.md
└── docs/
    └── design-doc.md             # 本设计文档
```

---

## 12. 开发路线

### Phase 1 - 核心骨架（v0.1.0）

- [x] 项目初始化（go.mod，基础结构）
- [ ] 配置系统：YAML 解析 + conf 解析 + 继承合并
- [ ] 日志系统：控制台 + 文件输出（线程安全）
- [ ] 同步引擎：基础文件复制

### Phase 2 - 并发监听与同步（v0.2.0）

- [ ] fsnotify 集成 + watcher goroutine 封装
- [ ] Debouncer goroutine
- [ ] Syncer Worker Pool
- [ ] Event Chan 路由（按文件 hash 分发）
- [ ] 首次全量同步（goroutine 池并行）
- [ ] Context 生命周期管理
- [ ] 优雅关闭（signal -> cancel -> wait）

### Phase 3 - 增强功能（v0.3.0）

- [ ] `--dry-run` 预览模式
- [ ] 触发式外部命令
- [ ] 热加载配置
- [ ] 同步状态报告
- [ ] Goroutine 监控与自动恢复

### Phase 4 - 扩展（v0.4.0+）

- [ ] Web 状态界面
- [ ] 远程同步
- [ ] 文件版本快照
- [ ] 更多...

---

## 附录 A: 配置示例

### sync.yaml（完整示例）

```yaml
log:
  level: info
  output: both
  file:
    path: ./logs/sync.log
    max_size: 50
    max_backups: 5
    compress: true

watch:
  debounce: 1000
  interval: 100
  workers: 0

sync:
  mode: incremental
  workers: 4
  backup: false
  verify: true
  exclude:
    - "*.tmp"
    - ".git/"
    - "*.swp"
  include: []

shutdown:
  timeout: 30

conf:
  dir: ./conf
  entry: sync.conf
```

### conf 文件示例（父子继承）

```conf
# conf/sync.conf - 入口配置
[task]
name = "前端资源同步"
source = "../web-app/dist"
target = "./deploy/public"
mode = "mirror"
delete_orphans = true
exclude = ["*.map"]

[inherit]
# 不继承，此为根配置
```

```conf
# conf/dev.conf - 开发环境覆盖
[task]
name = "前端资源同步（开发）"
target = "./deploy-dev/public"
exclude = ["*.map", "*.ts"]  # 追加到父 exclude 列表

[inherit]
file = "sync.conf"