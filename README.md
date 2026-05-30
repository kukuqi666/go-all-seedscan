# go-all-seedscan

一个基于 Go 编写的 BIP-39 助记词恢复与多链余额扫描工具。

它的用途是：

- 对包含缺失单词的助记词模板进行穷举恢复，缺失位置使用 `?` 表示
- 对候选助记词做 BIP-39 校验，快速过滤无效组合
- 按固定派生路径生成 EVM 地址
- 扫描多个 EVM 链上的原生币余额，找出非零余额钱包

当前代码支持以下链：

- Ethereum
- BNB Chain
- Polygon
- Avalanche C-Chain
- Arbitrum
- Optimism

## 工作原理

程序的大致流程如下：

1. 输入 12 或 24 个单词的助记词模板，未知单词用 `?` 占位。
2. 程序找出所有缺失位置，并将首层搜索任务分发到多个 worker 并行执行。
3. 每个候选组合先通过 `fastChecksumValid()` 做 SHA-256 校验位预过滤。
4. 通过预过滤的候选值再使用 BIP-39 完整校验，得到有效助记词。
5. 对每条有效助记词按 EVM 派生路径生成地址，并依次查询各条链的原生币余额。
6. 如果发现非零余额地址，就输出助记词、链名、地址和余额。

性能上，缺失单词每增加 1 个，搜索空间都会额外乘以 2048，因此缺失 3 个以上单词时耗时可能显著增加。

## 功能特性

- 支持恢复包含缺失单词的 12/24 词 BIP-39 助记词
- 支持直接校验完整助记词是否有效
- 默认使用 CPU 核心数并发搜索
- 使用快速校验位过滤，减少无效组合进入完整校验
- 支持在找到第一个非零余额钱包后提前停止
- 支持调试日志，便于观察恢复与扫描过程

## 环境要求

- Go `1.24.3` 或更高版本
- 可访问目标链 RPC 节点的网络环境

## 安装

### 方式一：直接运行

```sh
git clone <your-repo-url>
cd go-all-seedscan
go run main.go -s "word1 word2 ? word4 ... word12"
```

### 方式二：编译后运行

```sh
git clone <your-repo-url>
cd go-all-seedscan
go build -o go-all-seedscan
./go-all-seedscan -s "word1 word2 ? word4 ... word12"
```

## 使用方法

### 基本命令

```sh
./go-all-seedscan -s "word1 word2 ? word4 ... word12"
```

### 参数说明

| 参数 | 短参数 | 默认值 | 说明 |
|---|---|---|---|
| `--seed` | `-s` | 必填 | 助记词模板，缺失单词使用 `?` |
| `--max-word` | `-m` | `5` | 允许缺失的最大单词数量 |
| `--workers` | `-w` | CPU 核心数 | 并发 worker 数量 |
| `--debug` | `-d` | `false` | 开启调试日志 |
| `--stop-first` | `-f` | `true` | 找到第一个非零余额钱包后停止 |

### 使用示例

```sh
# 恢复 1 个缺失单词
./go-all-seedscan -s "word1 word2 ? word4 word5 word6 word7 word8 word9 word10 word11 word12"

# 恢复多个缺失单词，并指定并发数
./go-all-seedscan -s "word1 ? word3 ? word5 word6 word7 word8 word9 word10 word11 word12" -w 16

# 校验一条完整助记词
./go-all-seedscan -s "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12"

# 限制最大允许缺失单词数量
./go-all-seedscan -s "word1 ? ? word4 word5 word6 word7 word8 word9 word10 word11 word12" -m 2

# 开启调试日志
./go-all-seedscan -s "word1 word2 ? word4 word5 word6 word7 word8 word9 word10 word11 word12" -d
```

## 输出说明

当输入的是完整助记词时：

- 如果助记词有效，程序会输出“提供的助记词有效”
- 如果助记词无效，程序会输出“提供的助记词无效”

当输入包含 `?` 的模板时：

- 程序会先输出缺失单词数量、组合数量和并发 worker 数
- 找到非零余额钱包后，会输出助记词、链名、地址和余额
- 如果所有链都没有发现非零余额钱包，会输出失败提示

示例输出：

```text
2026-05-30T12:00:00Z INF 开始碰撞恢复 missing_words=1 combinations=2048 workers=8
2026-05-30T12:00:02Z INF ✅ 找到非零余额的钱包！ seed="word1 word2 ... word12" chain=Ethereum address=0x1234... balance=1000000000000000000
2026-05-30T12:00:02Z INF 程序结束 elapsed=2s
```

## 地址派生与扫描说明

- 助记词种子通过 `go-bip39` 与 `go-bip32` 生成主密钥
- 当前所有链都使用相同的 EVM 派生路径：`m/44'/60'/0'/0/0`
- 地址通过 `go-ethereum` 生成标准 EVM 地址
- 余额查询使用各链公开 RPC 节点，并读取地址原生币余额

这意味着：

- Ethereum 查询的是 ETH
- BNB Chain 查询的是 BNB
- Polygon 查询的是 MATIC
- Avalanche C-Chain 查询的是 AVAX
- Arbitrum 查询的是 ETH
- Optimism 查询的是 ETH

## 实现细节

项目中几个关键设计点：

- `fastChecksumValid()` 会在完整 BIP-39 校验前做一次快速校验位验证，显著减少无效组合
- 搜索任务按词表分块后分发到多个 worker，并避免共享可变切片带来的并发问题
- 余额检查阶段通过有界并发控制 RPC 请求数量，避免同时发起过多链上查询
- 启用 `--stop-first` 时，程序会在命中第一个非零余额钱包后取消后续搜索

## 注意事项

- 仅支持 12 或 24 词助记词
- 缺失单词越多，穷举成本越高，请合理设置 `--max-word`
- 当前仅扫描原生币余额，不检查 ERC-20、NFT 或其他资产
- 公开 RPC 可能存在限流、超时或不稳定问题，结果会受到网络环境影响
- 助记词极其敏感，请务必离线保存，不要在不可信环境中使用真实资产助记词

## 开发

```sh
go mod tidy
go run main.go -s "word1 word2 ? word4 word5 word6 word7 word8 word9 word10 word11 word12" -d
```

## 免责声明

本项目按“现状”提供，不对任何资金损失、误扫、漏扫或使用风险负责。请仅在你拥有合法权限的钱包与助记词上使用本工具。

## License

[MIT](./LICENSE)
