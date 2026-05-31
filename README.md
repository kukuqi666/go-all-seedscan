# go-all-seedscan

BIP-39 助记词恢复与多链余额扫描工具，基于 Go 编写。

## 功能

- 对包含缺失单词的助记词模板进行穷举恢复（缺失位置用 `?` 表示）
- BIP-39 校验位快速预过滤，减少无效组合进入完整校验
- 支持 EVM 链、Bitcoin、Solana、Cosmos 系等多链地址派生与余额扫描
- 实时单行进度显示（已用时间、预计剩余、速度、失败数），命中钱包即时输出
- 多 worker 并发搜索，找到首个非零余额钱包后可自动停止
- 支持选择指定链扫描，减少无效查询提升速度
- RPC 连接池复用，查询失败自动重试（3 次），避免限流漏检

## 支持链

### EVM 兼容链（14 条）

| 链 | 短名 | 原生币 | 派生路径 |
|---|---|---|---|
| Ethereum | `eth` | ETH | `m/44'/60'/0'/0/0` |
| BNB Chain | `bsc` | BNB | `m/44'/60'/0'/0/0` |
| Polygon | `polygon` | MATIC | `m/44'/60'/0'/0/0` |
| Avalanche C-Chain | `avax` | AVAX | `m/44'/60'/0'/0/0` |
| Arbitrum | `arb` | ETH | `m/44'/60'/0'/0/0` |
| Optimism | `op` | ETH | `m/44'/60'/0'/0/0` |
| Base | `base` | ETH | `m/44'/60'/0'/0/0` |
| zkSync Era | `zksync` | ETH | `m/44'/60'/0'/0/0` |
| Linea | `linea` | ETH | `m/44'/60'/0'/0/0` |
| Mantle | `mantle` | MNT | `m/44'/60'/0'/0/0` |
| Scroll | `scroll` | ETH | `m/44'/60'/0'/0/0` |
| Fantom | `ftm` | FTM | `m/44'/60'/0'/0/0` |
| Celo | `celo` | CELO | `m/44'/60'/0'/0/0` |
| Cronos | `cro` | CRO | `m/44'/60'/0'/0/0` |

### Bitcoin（3 种地址格式）

| 链 | 短名 | 原生币 | 派生路径 | 地址格式 |
|---|---|---|---|---|
| Bitcoin (Native SegWit) | `btc` | BTC | `m/84'/0'/0'/0/0` | bc1q... |
| Bitcoin (Nested SegWit) | `btc-p2sh` | BTC | `m/49'/0'/0'/0/0` | 3... |
| Bitcoin (Legacy) | `btc-legacy` | BTC | `m/44'/0'/0'/0/0` | 1... |

### Solana

| 链 | 短名 | 原生币 | 派生路径 |
|---|---|---|---|
| Solana | `sol` | SOL | `m/44'/501'/0'/0'` |

### Cosmos 系（2 条）

| 链 | 短名 | 原生币 | 派生路径 | 地址前缀 |
|---|---|---|---|---|
| Cosmos (ATOM) | `atom` | ATOM | `m/44'/118'/0'/0/0` | cosmos1... |
| Osmosis | `osmo` | OSMO | `m/44'/118'/0'/0/0` | osmo1... |

## 环境要求

- Go 1.24+
- 可访问目标链 RPC/API 节点的网络环境

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
| `--chains` | `-c` | `all` | 要扫描的链，逗号分隔（如 `eth,btc,sol`）或 `all` |
| `--debug` | `-d` | `false` | 开启调试日志 |
| `--stop-first` | `-f` | `true` | 找到第一个非零余额钱包后停止 |

### 示例

恢复 1 个缺失单词：

```sh
./recover -s "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon ? about"
```

恢复 2 个缺失单词，只查 ETH 和 BSC（推荐，速度更快）：

```sh
./recover -s "interest quality illegal above young off erosion repair trap dinosaur ? ?" -m 2 -c eth,bsc
```

只查 Bitcoin：

```sh
./recover -s "... ? ?" -m 2 -c btc
```

只查 Solana：

```sh
./recover -s "... ? ?" -m 2 -c sol
```

混合查 BSC + BTC + SOL：

```sh
./recover -s "... ? ?" -m 2 -c bsc,btc,sol
```

查所有 20 条链（默认）：

```sh
./recover -s "... ? ?" -m 2 -c all
```

指定 16 个 worker：

```sh
./recover -s "abandon ? abandon ? abandon abandon abandon abandon abandon abandon abandon about" -w 16
```

校验完整助记词是否有效：

```sh
./recover -s "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
```

不自动停止，扫描所有候选：

```sh
./recover -s "abandon ? abandon abandon abandon abandon abandon abandon abandon abandon abandon about" -f=false
```

### 速度参考

2 个缺失单词时（搜索空间约 419 万，通过校验的约 26 万条）：

| 链选择 | 每次查询数 | 预估速度 | 预估总时间 |
|--------|-----------|---------|-----------|
| `-c eth` | 1 条链 | ~60条/s | ~1 小时 |
| `-c eth,bsc` | 2 条链 | ~30条/s | ~2.5 小时 |
| `-c btc` | 1 条链 | ~30条/s | ~2.5 小时 |
| `-c sol` | 1 条链 | ~30条/s | ~2.5 小时 |
| `-c all` | 20 条链 | ~3条/s | ~24 小时 |

> 实际速度取决于网络和 RPC/API 响应，建议优先查目标链以缩短时间。

## 输出说明

启动时打印会话信息面板：

```
+---------------------------------- 会话信息 ----------------------------------+
| 模式         : BIP39 助记词恢复 + 多链余额扫描                               |
| 缺失单词数   : 2                                                             |
| 并发数       : 16                                                            |
| 搜索空间     : 4,194,304                                                     |
| 找到即停止   : true                                                          |
+------------------------------------------------------------------------------+
```

搜索过程中单行实时刷新进度：

**枚举阶段：**

```
[████████████░░░░░░░░] 60.0% 已用2m30s 预计~3m45s 速度30条/s 命中0
```

**枚举完成后进入检查阶段：**

```
[████████████░░░░░░░░] 枚举完成 检查200,000/262,000 76.3% 已用28m 预计~9m 速度151条/s 命中0
```

**有限流失败时显示失败数：**

```
[████████████░░░░░░░░] 60.0% 已用2m30s 预计~3m45s 速度30条/s 命中0 失败123
```

| 字段 | 含义 |
|---|---|
| `[████░░░░]` | 搜索/检查进度条 |
| `枚举完成` | 分支穷举已完成，正在检查候选助记词 |
| `检查X/Y` | 已检查 X 条 / 共 Y 条有效候选 |
| `60.0%` | 当前完成百分比 |
| `已用` | 已经运行的时间 |
| `预计` | 按当前速度估算的剩余时间 |
| `速度` | 每秒完成链上余额检查的助记词数 |
| `命中` | 发现非零余额的钱包数 |
| `失败` | RPC/API 查询失败次数（含重试后仍失败） |

命中钱包时输出结果面板：

```
+---------------------------------- 发现钱包 ----------------------------------+
| 链       : Bitcoin (BTC)                                                    |
| 地址     : bc1q...                                                          |
| 余额     : 1,000,000                                                        |
| 助记词   : abandon abandon ... about                                         |
+------------------------------------------------------------------------------+
```

输入完整助记词时，直接显示助记词是否有效。

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
  fastChecksumValid() 快速校验位预过滤
        │
        ▼
  bip39.IsMnemonicValid() 完整校验
        │
        ▼
  按链类型派生地址：
    EVM → deriveEVMAddress() (secp256k1)
    BTC → deriveBitcoinAddress() (P2WPKH/P2SH-P2WPKH/P2PKH)
    SOL → deriveSolanaAddress() (Ed25519)
    Cosmos → deriveCosmosAddress() (bech32)
        │
        ▼
  并发查询指定链余额（连接池复用 + 失败重试3次）
        │
        ▼
  输出非零余额结果
```

缺失单词每增加 1 个，搜索空间乘以 2048。缺失 3 个以上时耗时可能显著增加。

## 项目结构

```
go-all-seedscan/
├── main.go                      # 入口
├── internal/
│   ├── config/config.go         # 配置解析与校验
│   ├── chain/
│   │   ├── chains.go            # 链定义、短名映射与过滤
│   │   ├── evm.go               # EVM 地址派生、连接池与余额查询
│   │   ├── bitcoin.go           # Bitcoin 地址派生（3种格式）与余额查询
│   │   ├── solana.go            # Solana 地址派生（Ed25519）与余额查询
│   │   └── cosmos.go            # Cosmos 系地址派生（bech32）与余额查询
│   ├── core/
│   │   ├── mnemonic.go          # BIP39 词表与快速校验
│   │   └── recovery.go          # 碰撞引擎与结果处理
│   ├── ui/dashboard.go          # 终端 UI 渲染
│   └── logger/logger.go         # 日志初始化
├── go.mod
└── go.sum
```

核心碰撞引擎通过 `Reporter` 接口与 UI 解耦，可替换为其他输出方式（Web UI、API 等）而无需修改恢复逻辑。

## 注意事项

- 仅支持 12 或 24 词助记词
- 缺失单词越多穷举成本越高，合理设置 `--max-word`
- 使用 `--chains` 选择目标链可大幅缩短扫描时间
- 仅扫描原生币余额，不检查 ERC-20、SPL Token、NFT 或其他资产
- 公开 RPC/API 可能存在限流或不稳定，程序会自动重试 3 次
- 进度条中"失败"数较高时说明被限流，可减少并发数或链数
- 助记词极其敏感，请勿在不可信环境中使用真实资产助记词

## License

[MIT](./LICENSE)
