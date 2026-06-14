package core

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"recover/internal/chain"
	"recover/internal/config"

	"github.com/tyler-smith/go-bip39"
)

const ProgressInterval = 200 * time.Millisecond

type ProgressTracker struct {
	processedBranches atomic.Uint64
	validMnemonics    atomic.Uint64
	checkedMnemonics  atomic.Uint64
	hitsFound         atomic.Uint64
	failedChecks      atomic.Uint64
}

type Reporter interface {
	StartSession(missingWords int, searchSpace *big.Int, workers int, stopOnFirst bool, offline bool, targetAddress string)
	UpdateProgress(processedBranches, totalBranches uint64, estimatedProcessed, totalComb *big.Int, validMnemonics, checkedMnemonics, hitsFound, failedChecks uint64, elapsed time.Duration, status string)
	AddWalletHit(chainName, symbol, address, balance, seed string)
	SetMessage(message string)
	PrintValidation(valid bool)
	PrintSummary(elapsed time.Duration)
	PrintMnemonicList(mnemonics []string)
	Close()
}

func recoverInner(words []string, missing []int, depth int, results chan<- string, canceled func() bool, progress *ProgressTracker) {
	if canceled() {
		return
	}
	if depth >= len(missing) {
		if FastChecksumValid(words) {
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

func startProgressLoop(ctx context.Context, missingCount int, tracker *ProgressTracker, start time.Time, reporter Reporter) {
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
				elapsed := time.Since(start)
				processedBranches := tracker.processedBranches.Load()
				if processedBranches > totalBranches {
					processedBranches = totalBranches
				}
				estimatedProcessed := new(big.Int).Mul(new(big.Int).SetUint64(processedBranches), combPerBranch)
				if estimatedProcessed.Cmp(totalComb) > 0 {
					estimatedProcessed = new(big.Int).Set(totalComb)
				}
				reporter.UpdateProgress(
					processedBranches, totalBranches,
					estimatedProcessed, totalComb,
					tracker.validMnemonics.Load(),
					tracker.checkedMnemonics.Load(),
					tracker.hitsFound.Load(),
					tracker.failedChecks.Load(),
					elapsed,
					"任务已结束，等待结果汇总。",
				)
				return
			case <-ticker.C:
				elapsed := time.Since(start)
				processedBranches := tracker.processedBranches.Load()
				if processedBranches > totalBranches {
					processedBranches = totalBranches
				}
				estimatedProcessed := new(big.Int).Mul(new(big.Int).SetUint64(processedBranches), combPerBranch)
				if estimatedProcessed.Cmp(totalComb) > 0 {
					estimatedProcessed = new(big.Int).Set(totalComb)
				}
				reporter.UpdateProgress(
					processedBranches, totalBranches,
					estimatedProcessed, totalComb,
					tracker.validMnemonics.Load(),
					tracker.checkedMnemonics.Load(),
					tracker.hitsFound.Load(),
					tracker.failedChecks.Load(),
					elapsed,
					"正在搜索候选助记词并执行多链余额检查...",
				)
			}
		}
	}()
}

func RecoverSeedPhrase(cfg config.Config, reporter Reporter) {
	words := strings.Fields(cfg.SeedPhrase)
	if (len(words) == 12 || len(words) == 24) && !strings.Contains(cfg.SeedPhrase, "?") {
		if !cfg.Offline {
			reporter.StartSession(0, big.NewInt(1), cfg.Workers, cfg.StopOnFirst, cfg.Offline, "")
			reporter.PrintValidation(bip39.IsMnemonicValid(cfg.SeedPhrase))
			return
		}
		// 离线模式：本地派生地址并与目标地址比对
		reporter.StartSession(0, big.NewInt(1), cfg.Workers, cfg.StopOnFirst, cfg.Offline, cfg.TargetAddress)

		activeChains, err := chain.FilterChains(cfg.Chains)
		if err != nil {
			reporter.SetMessage(err.Error())
			return
		}
		activeChains = chain.ExpandChainIndices(activeChains, cfg.MaxAddrIndex)

		addrs := chain.DeriveMnemonicAddresses(cfg.SeedPhrase, activeChains)
		targets := parseTargets(cfg.TargetAddress)
		var found bool
		for _, am := range addrs {
			if targets[strings.ToLower(am.Address)] {
				found = true
				reporter.AddWalletHit(am.ChainName, am.Symbol, am.Address, "地址匹配", cfg.SeedPhrase)
			}
		}
		if found {
			reporter.SetMessage("扫描结束，已找到与目标地址匹配的助记词！")
		} else {
			reporter.SetMessage("扫描结束，未找到与目标地址匹配的助记词。")
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
		reporter.StartSession(0, big.NewInt(1), cfg.Workers, cfg.StopOnFirst, cfg.Offline, "")
		reporter.SetMessage("未发现缺失单词，无需碰撞。")
		return
	}
	if len(missing) > cfg.MaxWordMissing {
		reporter.StartSession(len(missing), big.NewInt(0), cfg.Workers, cfg.StopOnFirst, cfg.Offline, "")
		reporter.SetMessage(fmt.Sprintf("缺失单词数量 %d 超过限制 %d", len(missing), cfg.MaxWordMissing))
		return
	}

	totalComb := powInt(int64(len(wordList)), len(missing))

	activeChains, err := chain.FilterChains(cfg.Chains)
	if err != nil {
		reporter.StartSession(len(missing), totalComb, cfg.Workers, cfg.StopOnFirst, cfg.Offline, "")
		reporter.SetMessage(err.Error())
		return
	}

	activeChains = chain.ApplyCustomRPC(activeChains, cfg.CustomRPC)
	activeChains = chain.ExpandChainIndices(activeChains, cfg.MaxAddrIndex)

	reporter.StartSession(len(missing), totalComb, cfg.Workers, cfg.StopOnFirst, cfg.Offline, cfg.TargetAddress)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	progress := &ProgressTracker{}
	startProgressLoop(ctx, len(missing), progress, time.Now(), reporter)

	results := make(chan string, 4096)
	var wg sync.WaitGroup
	chunkSize := (len(wordList) + cfg.Workers - 1) / cfg.Workers
	for i := 0; i < cfg.Workers; i++ {
		start := i * chunkSize
		end := min(start+chunkSize, len(wordList))
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

	var allMnemonics []string
	for phrase := range results {
		allMnemonics = append(allMnemonics, phrase)
	}

	var found bool
	if cfg.Offline {
		found = matchAddresses(allMnemonics, activeChains, cfg.TargetAddress, cfg.StopOnFirst, progress, reporter)
	} else {
		found = checkBalances(allMnemonics, activeChains, cfg.StopOnFirst, progress, reporter)
	}

	cancel()

	if !found {
		if cfg.ShowAll && len(allMnemonics) > 0 {
			reporter.PrintMnemonicList(allMnemonics)
		}
		if cfg.Offline {
			reporter.SetMessage("扫描结束，未找到与目标地址匹配的助记词。")
		} else {
			reporter.SetMessage("扫描结束，在所有链上均未找到非零余额钱包。")
		}
	} else {
		if cfg.Offline {
			reporter.SetMessage("扫描结束，已找到与目标地址匹配的助记词！")
		} else {
			reporter.SetMessage("扫描结束，已找到至少一个非零余额钱包。")
		}
	}
}

func checkBalances(mnemonics []string, activeChains []chain.ChainConfig, stopOnFirst bool, progress *ProgressTracker, reporter Reporter) bool {
	var found atomic.Bool

	for _, phrase := range mnemonics {
		chainResults, failedChains, _ := chain.CheckMnemonicOnChains(phrase, activeChains)
		progress.checkedMnemonics.Add(1)

		if len(failedChains) > 0 {
			progress.failedChecks.Add(uint64(len(failedChains)))

			retryResults, retryFailed, _ := chain.CheckMnemonicOnChains(phrase, failedChains)
			if len(retryFailed) > 0 {
				progress.failedChecks.Add(uint64(len(retryFailed)))
			}
			chainResults = append(chainResults, retryResults...)
		}

		for _, res := range chainResults {
			progress.hitsFound.Add(1)
			found.Store(true)
			reporter.AddWalletHit(res.ChainName, res.Symbol, res.Address, res.Balance.String(), phrase)
		}

		if found.Load() && stopOnFirst {
			return true
		}
	}

	return found.Load()
}

func parseTargets(targetAddress string) map[string]bool {
	targets := make(map[string]bool)
	for _, addr := range strings.Split(targetAddress, ",") {
		addr = strings.TrimSpace(addr)
		if addr != "" {
			targets[strings.ToLower(addr)] = true
		}
	}
	return targets
}

func matchAddresses(mnemonics []string, activeChains []chain.ChainConfig, targetAddress string, stopOnFirst bool, progress *ProgressTracker, reporter Reporter) bool {
	var found atomic.Bool

	targets := parseTargets(targetAddress)

	for _, phrase := range mnemonics {
		addresses := chain.DeriveMnemonicAddresses(phrase, activeChains)
		progress.checkedMnemonics.Add(1)

		for _, am := range addresses {
			if targets[strings.ToLower(am.Address)] {
				progress.hitsFound.Add(1)
				found.Store(true)
				reporter.AddWalletHit(am.ChainName, am.Symbol, am.Address, "地址匹配", phrase)
			}
		}

		if found.Load() && stopOnFirst {
			return true
		}
	}

	return found.Load()
}

func powInt(base int64, exp int) *big.Int {
	result := big.NewInt(1)
	if exp <= 0 {
		return result
	}

	b := big.NewInt(base)
	for range exp {
		result.Mul(result, b)
	}
	return result
}
