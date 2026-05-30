package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bytedance/sonic"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

// ─────────────────── 多链配置 ─────────────────────────────────────

type ChainConfig struct {
	Name           string // 链名称
	Symbol         string // 原生币符号
	DerivationPath string // BIP44 派生路径
	RPC            string // JSON-RPC 地址
	Type           string // "evm"
}

var chains = []ChainConfig{
	{
		Name:           "Ethereum",
		Symbol:         "ETH",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://eth.llamarpc.com",
		Type:           "evm",
	},
	{
		Name:           "BNB Chain",
		Symbol:         "BNB",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://bsc-dataseed.binance.org",
		Type:           "evm",
	},
	{
		Name:           "Polygon",
		Symbol:         "MATIC",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://polygon-rpc.com",
		Type:           "evm",
	},
	{
		Name:           "Avalanche C-Chain",
		Symbol:         "AVAX",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://api.avax.network/ext/bc/C/rpc",
		Type:           "evm",
	},
	{
		Name:           "Arbitrum",
		Symbol:         "ARB",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://arb1.arbitrum.io/rpc",
		Type:           "evm",
	},
	{
		Name:           "Optimism",
		Symbol:         "OP",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://mainnet.optimism.io",
		Type:           "evm",
	},
}

const (
	MaxAPIConcurrency = 10
	APITimeout        = 10 * time.Second
	ProgressInterval  = 2 * time.Second
	PanelWidth        = 70
)

// ─────────────────── 配置 ─────────────────────────────────────────

type Config struct {
	SeedPhrase     string
	MaxWordMissing int
	Workers        int
	Debug          bool
	StopOnFirst    bool
}

func parseConfig() Config {
	var cfg Config
	pflag.StringVarP(&cfg.SeedPhrase, "seed", "s", "", "助记词模板，缺失单词用 ? 表示")
	pflag.IntVarP(&cfg.MaxWordMissing, "max-word", "m", 5, "允许缺失的最大单词数量")
	pflag.IntVarP(&cfg.Workers, "workers", "w", runtime.NumCPU(), "并发 worker 数量")
	pflag.BoolVarP(&cfg.Debug, "debug", "d", false, "开启调试日志")
	pflag.BoolVarP(&cfg.StopOnFirst, "stop-first", "f", true, "找到第一个有余额的钱包后停止")
	pflag.Parse()
	return cfg
}

func validateConfig(cfg Config) error {
	if cfg.SeedPhrase == "" {
		return fmt.Errorf("助记词模板不能为空")
	}
	if cfg.MaxWordMissing < 0 || cfg.MaxWordMissing > 24 {
		return fmt.Errorf("max-word 必须在 0-24 之间")
	}
	if cfg.Workers <= 0 {
		return fmt.Errorf("workers 必须大于0")
	}
	return nil
}

// ─────────────────── 日志 ─────────────────────────────────────────

func initLogger(debug bool) {
	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.InterfaceMarshalFunc = sonic.Marshal
	zerolog.CallerMarshalFunc = func(_ uintptr, file string, line int) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}
	log.Logger = zerolog.New(&zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
		NoColor:    false,
	}).With().Timestamp().Caller().Logger()
}

// ─────────────────── 地址派生（EVM）────────────────────────────────

func deriveEVMAddress(mnemonic, pathStr string) (string, error) {
	seed := bip39.NewSeed(mnemonic, "")
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return "", errors.Wrap(err, "生成主密钥失败")
	}

	parts := strings.Split(pathStr, "/")
	if len(parts) < 2 || parts[0] != "m" {
		return "", fmt.Errorf("无效的路径格式: %s", pathStr)
	}

	currentKey := masterKey
	for _, part := range parts[1:] {
		var childNum uint32
		if strings.HasSuffix(part, "'") {
			numStr := strings.TrimSuffix(part, "'")
			num, err := strconv.ParseUint(numStr, 10, 32)
			if err != nil {
				return "", errors.Wrapf(err, "解析路径数字失败: %s", part)
			}
			childNum = uint32(num) + bip32.FirstHardenedChild
		} else {
			num, err := strconv.ParseUint(part, 10, 32)
			if err != nil {
				return "", errors.Wrapf(err, "解析路径数字失败: %s", part)
			}
			childNum = uint32(num)
		}
		currentKey, err = currentKey.NewChildKey(childNum)
		if err != nil {
			return "", errors.Wrapf(err, "派生子密钥失败: %s", part)
		}
	}

	privateKey, err := crypto.ToECDSA(currentKey.Key)
	if err != nil {
		return "", errors.Wrap(err, "转换为 ECDSA 私钥失败")
	}
	address := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	return address, nil
}

// ─────────────────── 余额查询（EVM）────────────────────────────────

func checkEVMBalance(address string, rpcURL string) (*big.Int, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, errors.Wrap(err, "连接 RPC 失败")
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
	defer cancel()

	balance, err := client.BalanceAt(ctx, common.HexToAddress(address), nil)
	if err != nil {
		return nil, errors.Wrap(err, "查询余额失败")
	}
	return balance, nil
}

// ─────────────────── 多链检查 ─────────────────────────────────────

type ChainResult struct {
	ChainName string
	Symbol    string
	Balance   *big.Int
	Address   string
}

type ProgressTracker struct {
	processedBranches atomic.Uint64
	validMnemonics    atomic.Uint64
	checkedMnemonics  atomic.Uint64
	foundBannerShown  atomic.Bool
}

func checkMnemonicOnAllChains(mnemonic string) ([]ChainResult, error) {
	var results []ChainResult
	for _, chain := range chains {
		if chain.Type != "evm" {
			continue
		}
		addr, err := deriveEVMAddress(mnemonic, chain.DerivationPath)
		if err != nil {
			log.Debug().Err(err).Str("chain", chain.Name).Msg("地址派生失败")
			continue
		}
		balance, err := checkEVMBalance(addr, chain.RPC)
		if err != nil {
			log.Debug().Err(err).Str("chain", chain.Name).Str("addr", addr).Msg("余额查询失败")
			continue
		}
		if balance != nil && balance.Cmp(big.NewInt(0)) > 0 {
			results = append(results, ChainResult{
				ChainName: chain.Name,
				Symbol:    chain.Symbol,
				Balance:   balance,
				Address:   addr,
			})
		}
	}
	return results, nil
}

// ─────────────────── BIP39 快速校验 ───────────────────────────────

var (
	wordList    []string
	wordListMap map[string]int
)

func initWordList() {
	wordList = bip39.GetWordList()
	wordListMap = make(map[string]int, len(wordList))
	for i, w := range wordList {
		wordListMap[w] = i
	}
}

func fastChecksumValid(words []string) bool {
	n := len(words)
	if n != 12 && n != 24 {
		return false
	}
	totalBits := n * 11
	entropyBits := totalBits - totalBits/32
	checksumBits := totalBits - entropyBits
	entropyBytes := entropyBits / 8
	totalBytes := (totalBits + 7) / 8

	buf := make([]byte, totalBytes)
	bitPos := 0
	for _, w := range words {
		idx, ok := wordListMap[w]
		if !ok {
			return false
		}
		for i := 10; i >= 0; i-- {
			byteIdx := bitPos / 8
			bitIdx := 7 - (bitPos % 8)
			if idx&(1<<i) != 0 {
				buf[byteIdx] |= 1 << bitIdx
			}
			bitPos++
		}
	}
	entropy := buf[:entropyBytes]
	var checksum byte = 0
	for i := 0; i < checksumBits; i++ {
		byteIdx := (entropyBytes*8 + i) / 8
		bitIdx := 7 - ((entropyBytes*8 + i) % 8)
		if (buf[byteIdx]>>bitIdx)&1 != 0 {
			checksum |= 1 << (7 - i)
		}
	}
	checksum >>= (8 - checksumBits)
	hash := sha256.Sum256(entropy)
	return (hash[0] >> (8 - checksumBits)) == checksum
}

// ─────────────────── 碰撞引擎 ─────────────────────────────────────

func recoverInner(words []string, missing []int, depth int, results chan<- string, canceled func() bool, progress *ProgressTracker) {
	if canceled() {
		return
	}
	if depth >= len(missing) {
		if fastChecksumValid(words) {
			phrase := strings.Join(words, " ")
			if bip39.IsMnemonicValid(phrase) {
				progress.validMnemonics.Add(1)
				results <- phrase
			}
		}
		return
	}
	pos := missing[depth]
	for _, w := range wordList {
		words[pos] = w
		recoverInner(words, missing, depth+1, results, canceled, progress)
		if canceled() {
			return
		}
	}
}

func powInt(base int64, exp int) *big.Int {
	result := big.NewInt(1)
	if exp <= 0 {
		return result
	}
	b := big.NewInt(base)
	for i := 0; i < exp; i++ {
		result.Mul(result, b)
	}
	return result
}

func formatBigInt(n *big.Int) string {
	if n == nil {
		return "0"
	}
	return formatNumberString(n.String())
}

func formatNumberString(s string) string {
	if len(s) <= 3 {
		return s
	}

	sign := ""
	if strings.HasPrefix(s, "-") {
		sign = "-"
		s = s[1:]
	}

	rem := len(s) % 3
	if rem == 0 {
		rem = 3
	}

	var b strings.Builder
	b.Grow(len(s) + len(s)/3)
	b.WriteString(sign)
	b.WriteString(s[:rem])
	for i := rem; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func formatUint64(v uint64) string {
	return formatNumberString(strconv.FormatUint(v, 10))
}

func formatDurationShort(d time.Duration) string {
	if d < time.Second {
		return d.Truncate(time.Millisecond).String()
	}
	if d < time.Minute {
		return d.Truncate(time.Second).String()
	}
	return d.Truncate(time.Second).String()
}

func renderProgressBar(percent float64, width int) string {
	if width <= 0 {
		width = 24
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + "]"
}

func trimToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return s[:width-3] + "..."
}

func panelBorder() string {
	return "+" + strings.Repeat("-", PanelWidth-2) + "+"
}

func panelTitle(title string) string {
	title = " " + trimToWidth(title, PanelWidth-6) + " "
	fill := PanelWidth - 2 - len(title)
	left := fill / 2
	right := fill - left
	return "+" + strings.Repeat("-", left) + title + strings.Repeat("-", right) + "+"
}

func panelLine(content string) string {
	content = trimToWidth(content, PanelWidth-4)
	return fmt.Sprintf("| %-*s |", PanelWidth-4, content)
}

func printPanel(title string, lines ...string) {
	fmt.Fprintln(os.Stdout, panelTitle(title))
	for _, line := range lines {
		fmt.Fprintln(os.Stdout, panelLine(line))
	}
	fmt.Fprintln(os.Stdout, panelBorder())
}

func printStartupBanner(missingCount int, totalComb *big.Int, workers int) {
	banner := `
+------------------------------------------------------------------+
|   ____  ___         _    _     _       ____                      |
|  / ___|/ _ \       / \  | |   | |     / ___|  ___  __ _ _ __     |
| | |  _| | | |     / _ \ | |   | |_____\___ \ / __|/ _` + "`" + ` | '_ \    |
| | |_| | |_| |    / ___ \| |___| |_____|___) | (__| (_| | | | |   |
|  \____|\___/    /_/   \_\_____|_|     |____/ \___|\__,_|_| |_|   |
+------------------------------------------------------------------+
| Seed Recovery Dashboard                                          |
| Missing Words : %-47d |
| Workers       : %-47d |
| Search Space  : %-47s |
+------------------------------------------------------------------+
`

	fmt.Fprintf(os.Stdout, banner+"\n", missingCount, workers, formatBigInt(totalComb))
	printPanel(
		"SESSION",
		"Mode         : BIP39 Recovery + Multi-chain EVM Scan",
		fmt.Sprintf("Missing Words: %d", missingCount),
		fmt.Sprintf("Workers      : %d", workers),
		fmt.Sprintf("Search Space : %s", formatBigInt(totalComb)),
	)
}

func printFoundBanner() {
	banner := `
+----------------------------------------------------------+
| __        ___    _     _     _____ _____   _   _ ___ _____ |
| \ \      / / \  | |   | |   | ____|_   _| | | | |_ _|_   _||
|  \ \ /\ / / _ \ | |   | |   |  _|   | |   | |_| || |  | |  |
|   \ V  V / ___ \| |___| |___| |___  | |   |  _  || |  | |  |
|    \_/\_/_/   \_\_____|_____|_____| |_|   |_| |_|___| |_|  |
+----------------------------------------------------------+
`

	fmt.Fprintln(os.Stdout, banner)
}

func printWalletHitPanel(seed string, res ChainResult) {
	printPanel(
		"WALLET HIT",
		fmt.Sprintf("Chain   : %s", res.ChainName),
		fmt.Sprintf("Symbol  : %s", res.Symbol),
		fmt.Sprintf("Address : %s", res.Address),
		fmt.Sprintf("Balance : %s", formatBigInt(res.Balance)),
		fmt.Sprintf("Seed    : %s", seed),
	)
}

func startProgressLogger(ctx context.Context, missingCount int, tracker *ProgressTracker, start time.Time) {
	if missingCount <= 0 {
		return
	}

	totalBranches := uint64(len(wordList))
	combPerBranch := powInt(int64(len(wordList)), missingCount-1)
	totalComb := new(big.Int).Mul(new(big.Int).SetUint64(totalBranches), combPerBranch)

	ticker := time.NewTicker(ProgressInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processedBranches := tracker.processedBranches.Load()
				validMnemonics := tracker.validMnemonics.Load()
				checkedMnemonics := tracker.checkedMnemonics.Load()

				if processedBranches > totalBranches {
					processedBranches = totalBranches
				}

				progressPct := float64(processedBranches) / float64(totalBranches) * 100
				estimatedProcessed := new(big.Int).Mul(new(big.Int).SetUint64(processedBranches), combPerBranch)
				if estimatedProcessed.Cmp(totalComb) > 0 {
					estimatedProcessed = new(big.Int).Set(totalComb)
				}

				elapsed := time.Since(start)
				bar := renderProgressBar(progressPct, 32)
				checkedRate := float64(checkedMnemonics) / max(elapsed.Seconds(), 0.001)
				printPanel(
					"RECOVERY PROGRESS",
					fmt.Sprintf("%s %5.1f%%", bar, progressPct),
					fmt.Sprintf("Branches : %s / %s", formatUint64(processedBranches), formatUint64(totalBranches)),
					fmt.Sprintf("Search   : %s / %s", formatBigInt(estimatedProcessed), formatBigInt(totalComb)),
					fmt.Sprintf("Valid    : %s", formatUint64(validMnemonics)),
					fmt.Sprintf("Checked  : %s", formatUint64(checkedMnemonics)),
					fmt.Sprintf("Speed    : %.1f / sec", checkedRate),
					fmt.Sprintf("Elapsed  : %s", formatDurationShort(elapsed)),
				)
			}
		}
	}()
}

func RecoverSeedPhrase(cfg Config) {
	words := strings.Split(cfg.SeedPhrase, " ")
	if (len(words) == 12 || len(words) == 24) && !strings.Contains(cfg.SeedPhrase, "?") {
		if bip39.IsMnemonicValid(cfg.SeedPhrase) {
			log.Info().Msg("✅ 提供的助记词有效！")
		} else {
			log.Error().Msg("❌ 提供的助记词无效！")
		}
		return
	}
	var missing []int
	for i, w := range words {
		if w == "?" {
			missing = append(missing, i)
		}
	}
	if len(missing) == 0 {
		log.Info().Msg("未发现缺失单词，无需碰撞。")
		return
	}
	if len(missing) > cfg.MaxWordMissing {
		log.Error().Msgf("❌ 缺失单词数量 %d 超过限制 %d", len(missing), cfg.MaxWordMissing)
		return
	}
	totalComb := powInt(int64(len(wordList)), len(missing))
	printStartupBanner(len(missing), totalComb, cfg.Workers)
	log.Info().
		Int("missing_words", len(missing)).
		Str("combinations", formatBigInt(totalComb)).
		Int("workers", cfg.Workers).
		Msg("开始碰撞恢复")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	progress := &ProgressTracker{}
	startProgressLogger(ctx, len(missing), progress, time.Now())

	results := make(chan string, 256)
	var wg sync.WaitGroup
	chunkSize := (len(wordList) + cfg.Workers - 1) / cfg.Workers
	for i := 0; i < cfg.Workers; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(wordList) {
			end = len(wordList)
		}
		if start >= len(wordList) {
			break
		}
		wg.Add(1)
		go func(subset []string) {
			defer wg.Done()
			localWords := make([]string, len(words))
			copy(localWords, words)
			for _, firstWord := range subset {
				if ctx.Err() != nil {
					return
				}
				localWords[missing[0]] = firstWord
				recoverInner(localWords, missing, 1, results, func() bool { return ctx.Err() != nil }, progress)
				progress.processedBranches.Add(1)
			}
		}(wordList[start:end])
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	processResults(ctx, cancel, results, cfg.StopOnFirst, progress)
}

func processResults(ctx context.Context, cancel context.CancelFunc, results <-chan string, stopOnFirst bool, progress *ProgressTracker) {
	sem := make(chan struct{}, MaxAPIConcurrency)
	var found atomic.Bool
	var mu sync.Mutex
	var wg sync.WaitGroup

	for phrase := range results {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			chainResults, err := checkMnemonicOnAllChains(p)
			progress.checkedMnemonics.Add(1)
			if err != nil {
				log.Debug().Err(err).Msg("多链检查失败")
				return
			}
			if len(chainResults) > 0 {
				if progress.foundBannerShown.CompareAndSwap(false, true) {
					printFoundBanner()
				}
				mu.Lock()
				for _, res := range chainResults {
					printWalletHitPanel(p, res)
					log.Info().
						Str("seed", p).
						Str("chain", res.ChainName).
						Str("address", res.Address).
						Str("balance", res.Balance.String()).
						Msg("✅ 找到非零余额的钱包！")
				}
				mu.Unlock()
				found.Store(true)
				if stopOnFirst {
					cancel()
				}
			}
		}(phrase)
	}
	wg.Wait()
	if !found.Load() {
		log.Error().Msg("❌ 在所有链上均未找到有余额的钱包。")
	}
}

// ─────────────────── 主函数 ───────────────────────────────────────

func main() {
	cfg := parseConfig()
	initLogger(cfg.Debug)
	if err := validateConfig(cfg); err != nil {
		log.Fatal().Err(err).Msg("配置无效")
	}
	initWordList()
	start := time.Now()
	RecoverSeedPhrase(cfg)
	log.Info().Dur("elapsed", time.Since(start)).Msg("程序结束")
}
