# go-all-seedscan

BIP-39 助记词恢复与多链余额扫描工具，基于 Go 编写。

## 功能

- 对包含缺失单词的助记词模板进行穷举恢复（缺失位置用 `?` 表示）
- BIP-39 校验位快速预过滤，减少无效组合进入完整校验
- 支持 14 条 EVM 链、3 种 Bitcoin 格式、Solana、Cosmos 系等多链地址派生与余额扫描
- 支持离线地址比对模式（本地派生地址，无需 API，仅需已知目标地址）
- 支持 `--show-all` 列出所有有效候选助记词（手动验证用）
- 实时单行进度显示（已用时间、预计剩余、速度、失败数），命中钱包即时输出
- 多 worker 并发搜索，找到首个非零余额钱包后可自动停止
- 支持选择指定链扫描，减少无效查询提升速度
- RPC 连接池复用，查询失败自动重试（3 次），避免限流漏检
- 支持 BscScan / Etherscan 等区块浏览器 API Key，大幅提升稳定性

### 关于 2 个及以上缺失单词

程序支持缺失 1~5 个单词的恢复。缺失单词每增加 1 个，搜索空间乘以 2048：

| 缺失单词数 | 搜索空间 | 有效候选 ~ | 离线模式耗时 | 在线模式耗时 |
|-----------|---------|-----------|-------------|-------------|
| 1 | 2,048 | 128 | < 1s | ~4-9m（取决于链数）|
| 2 | 4,194,304 | 262,144 | ~30s | ~数小时 |
| 3 | 8,589,934,592 | ~536,870,912 | 不可行 | 不可行 |

> 2 个缺失单词时推荐使用离线模式（`-a` 参数），比在线模式快几个数量级。

## 支持链

### EVM 兼容链（14 条）— 共享派生路径 `m/44'/60'/0'/0/0`

| 链 | 短名 | 原生币 |
|---|---|---|
| Ethereum | `eth` | ETH |
| BNB Chain | `bsc` | BNB |
| Polygon | `polygon` | MATIC |
| Avalanche C-Chain | `avax` | AVAX |
| Arbitrum | `arb` | ETH |
| Optimism | `op` | ETH |
| Base | `base` | ETH |
| zkSync Era | `zksync` | ETH |
| Linea | `linea` | ETH |
| Mantle | `mantle` | MNT |
| Scroll | `scroll` | ETH |
| Fantom | `ftm` | FTM |
| Celo | `celo` | CELO |
| Cronos | `cro` | CRO |

### Bitcoin（3 种地址格式）

| 链 | 短名 | 派生路径 | 地址格式 |
|---|---|---|---|
| Native SegWit | `btc` | `m/84'/0'/0'/0/0` | bc1q... |
| Nested SegWit | `btc-p2sh` | `m/49'/0'/0'/0/0` | 3... |
| Legacy | `btc-legacy` | `m/44'/0'/0'/0/0` | 1... |

### Solana + Cosmos

| 链 | 短名 | 派生路径 | 地址前缀 |
|---|---|---|---|
| Solana | `sol` | `m/44'/501'/0'/0'` | — |
| Cosmos (ATOM) | `atom` | `m/44'/118'/0'/0/0` | cosmos1... |
| Osmosis | `osmo` | `m/44'/118'/0'/0/0` | osmo1... |

## 环境要求

- Go 1.24+
- 可访问目标链 RPC/API 节点的网络环境（在线模式需要）

## 安装

```sh
git clone https://github.com/kukuqi666/go-all-seedscan.git
cd go-all-seedscan
go build -o recover
```

或直接运行：

```sh
go run main.go -s "word1 word2 ? word4 ... word12"
```

## 使用方法

### 参数

| 参数 | 短参数 | 默认值 | 说明 |
|---|---|---|---|
| `--seed` | `-s` | 必填 | 助记词模板，缺失单词用 `?` 占位 |
| `--max-word` | `-m` | `5` | 允许缺失的最大单词数量 |
| `--workers` | `-w` | CPU 核心数 | 并发 worker 数量 |
| `--chains` | `-c` | `all` | 要扫描的链，逗号分隔 |
| `--address` | `-a` | — | 目标钱包地址（离线模式） |
| `--show-all` | — | `false` | 列出所有有效候选助记词 |
| `--stop-first` | `-f` | `true` | 找到第一个匹配后停止 |
| `--api-key` | `-k` | — | 区块浏览器 API Key |
| `--rpc` | `-r` | — | 自定义 RPC 端点 |
| `--addr-index` | `-i` | `4` | 检查的地址索引范围 0~N |
| `--debug` | `-d` | `false` | 开启调试日志 |

### 三种工作模式

#### 1. 离线地址匹配（最快，推荐）

已知目标钱包地址时，本地派生地址进行比对，无需网络请求：

```sh
# 缺失 1 个单词 + 已知 BNB 链地址
./recover -s "word1 word2 ? word4 ... word12" -c bsc -a "0x...目标地址"

# 缺失 2 个单词 + 已知多个链地址
./recover -s "word1 ? ? word4 ... word12" -m 2 -c bsc,eth -a "0x...地址1,0x...地址2"
```

#### 2. 在线余额扫描

无目标地址时，实时查询链上余额寻找非零钱包：

```sh
./recover -s "word1 ? word3 ... word12" -c bsc
./recover -s "word1 ? word3 ... word12" -c eth,bsc,btc
```

#### 3. `--show-all` 列表模式

无余额且无目标地址时，枚举所有有效候选助记词供手动验证：

```sh
# 终端直接查看
./recover -s "word1 ? word3 ... word12" -c bsc --show-all

# 保存到文件
./recover -s "word1 ? word3 ... word12" -c bsc --show-all > candidates.txt

# 用 grep 快速定位包含特定单词的组合
./recover -s "word1 ? word3 ... word12" --show-all | grep "known_word"
```

### 示例

```sh
# 恢复 1 个缺失单词（离线模式）
./recover -s "word1 word2 ? word4 ... word12" -c bsc -a "0x1234..."

# 恢复 2 个缺失单词（离线模式）
./recover -s "word1 word2 ? ? word5 ... word12" -m 2 -c eth,bsc -a "0x1234..."

# 校验完整助记词
./recover -s "word1 word2 word3 ... word12"

# 校验 + 地址验证
./recover -s "word1 word2 word3 ... word12" -c bsc -a "0x1234..."

# 指定 16 个 worker
./recover -s "word1 ? word3 ... word12" -w 16

# 指定 API Key 提升稳定性
./recover -s "word1 ? word3 ... word12" -c eth -k "YOUR_ETHERSCAN_API_KEY"
```

## 输出说明

启动时打印会话信息面板：

```
+---------------------------------- 会话信息 ---------------------------------- +
| 模式         : BIP39 助记词恢复 + 离线地址比对（无需API）                     |
| 缺失单词数   : 1                                                             |
| 并发数       : 16                                                            |
| 搜索空间     : 2,048                                                         |
| 找到即停止   : true                                                          |
| 目标地址     : 0x1234...                                                     |
+------------------------------------------------------------------------------ +
```

搜索过程中单行实时刷新进度：枚举阶段显示分支遍历进度，枚举完成后显示候选助记词检查进度。

命中钱包时输出结果面板：

```
+---------------------------------- 发现钱包 ---------------------------------- +
| 链       : Ethereum (ETH)                                                     |
| 地址     : 0x1234...                                                          |
| 余额     : 1.5 ETH                                                            |
| 助记词   : word1 word2 word3 ... word12                                       |
+------------------------------------------------------------------------------ +
```

## 工作原理

```
输入助记词模板（? 占位）
        │
        ▼
  定位缺失位置，计算搜索空间
        │
        ▼
  按词表分块 → 多 worker 并发
        │
        ▼
  递归穷举每个缺失位置
        │
        ▼
  FastChecksumValid() 快速校验位预过滤
        │
        ▼
  bip39.IsMnemonicValid() 完整校验
        │
        ▼
  [离线模式] 派生地址 → 比对目标地址
  [在线模式] 派生地址 → 查询链上余额
  [show-all]  收集所有候选 → 输出列表
        │
        ▼
  输出匹配结果
```

## 项目结构

```
go-all-seedscan/
├── main.go                      # 入口
├── internal/
│   ├── config/config.go         # 配置解析与校验
│   ├── chain/
│   │   ├── chains.go            # 链定义、短名映射与过滤
│   │   ├── evm.go               # EVM 地址派生、连接池与余额查询
│   │   ├── bitcoin.go           # Bitcoin 地址派生与余额查询
│   │   ├── solana.go            # Solana 地址派生与余额查询
│   │   └── cosmos.go            # Cosmos 系地址派生与余额查询
│   ├── core/
│   │   ├── mnemonic.go          # BIP39 词表与快速校验
│   │   └── recovery.go          # 碰撞引擎与结果处理
│   ├── ui/dashboard.go          # 终端 UI 渲染
│   └── logger/logger.go         # 日志初始化
├── go.mod
└── go.sum
```

## 注意事项

- 仅支持 12 或 24 词助记词
- 缺失单词越多穷举成本越高，合理设置 `--max-word`
- 使用 `--chains` 选择目标链可大幅缩短扫描时间
- **强烈推荐使用离线模式**（`-a`），速度快且无需网络
- 无地址且无余额时，使用 `--show-all` + 手动验证
- 仅扫描原生币余额，不检查 ERC-20、SPL Token、NFT 或其他资产
- 公开 RPC/API 可能存在限流或不稳定，程序会自动重试 3 次
- 助记词极其敏感，请勿在不可信环境中使用真实资产助记词

