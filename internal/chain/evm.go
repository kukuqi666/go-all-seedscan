package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

var globalAPIKey string

func SetAPIKey(key string) {
	globalAPIKey = key
}

var (
	clientMu   sync.RWMutex
	clientPool map[string]*ethclient.Client
)

func init() {
	clientPool = make(map[string]*ethclient.Client)
}

func getRPCClient(rpcURL string) (*ethclient.Client, error) {
	clientMu.RLock()
	if c, ok := clientPool[rpcURL]; ok {
		clientMu.RUnlock()
		return c, nil
	}
	clientMu.RUnlock()

	clientMu.Lock()
	defer clientMu.Unlock()

	if c, ok := clientPool[rpcURL]; ok {
		return c, nil
	}

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, errors.Wrap(err, "连接 RPC 失败")
	}
	clientPool[rpcURL] = client
	return client, nil
}

func DeriveEVMAddress(mnemonic, pathStr string) (string, error) {
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
		if numStr, hardened := strings.CutSuffix(part, "'"); hardened {
			num, parseErr := strconv.ParseUint(numStr, 10, 32)
			if parseErr != nil {
				return "", errors.Wrapf(parseErr, "解析路径数字失败: %s", part)
			}
			childNum = uint32(num) + bip32.FirstHardenedChild
		} else {
			num, parseErr := strconv.ParseUint(part, 10, 32)
			if parseErr != nil {
				return "", errors.Wrapf(parseErr, "解析路径数字失败: %s", part)
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

	return crypto.PubkeyToAddress(privateKey.PublicKey).Hex(), nil
}

func CheckEVMBalance(address string, rpcURLs []string, explorerAPI string) (*big.Int, error) {
	if globalAPIKey != "" && explorerAPI != "" {
		balance, err := CheckExplorerBalance(address, explorerAPI)
		if err == nil {
			return balance, nil
		}
		log.Debug().Err(err).Str("addr", address).Msg("Explorer API 查询失败，回退到 RPC")
	}
	return CheckRPCBalance(address, rpcURLs)
}

func CheckExplorerBalance(address, explorerAPI string) (*big.Int, error) {
	const maxRetries = 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}

		url := fmt.Sprintf("%s?module=account&action=balance&address=%s&apikey=%s",
			explorerAPI, address, globalAPIKey)

		acquireAPI()
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			cancel()
			releaseAPI()
			continue
		}
		req.Header.Set("User-Agent", "seedscan/1.0")

		resp, err := http.DefaultClient.Do(req)
		cancel()
		releaseAPI()

		if err != nil {
			continue
		}

		if resp.StatusCode == 429 {
			resp.Body.Close()
			time.Sleep(2 * time.Second)
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		var result struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Result  string `json:"result"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		if result.Status != "1" {
			continue
		}

		balance, ok := new(big.Int).SetString(result.Result, 10)
		if !ok {
			continue
		}

		return balance, nil
	}

	return nil, fmt.Errorf("Explorer API 查询余额失败（重试%d次）", maxRetries)
}

func CheckRPCBalance(address string, rpcURLs []string) (*big.Int, error) {
	const maxRetries = 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 800 * time.Millisecond)
		}

		rpcURL := rpcURLs[attempt%len(rpcURLs)]

		acquireAPI()
		time.Sleep(MinAPIDelay)
		client, err := getRPCClient(rpcURL)
		if err != nil {
			releaseAPI()
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		balance, err := client.BalanceAt(ctx, common.HexToAddress(address), nil)
		cancel()
		releaseAPI()

		if err != nil {
			clientMu.Lock()
			delete(clientPool, rpcURL)
			clientMu.Unlock()
			continue
		}

		return balance, nil
	}

	return nil, fmt.Errorf("查询余额失败（重试%d次）", maxRetries)
}

func CheckMnemonicOnChains(mnemonic string, chains []ChainConfig) (results []ChainResult, failedChains []ChainConfig, err error) {
	type chainCheckResult struct {
		res    ChainResult
		ok     bool
		failed bool
		chain  ChainConfig
	}

	ch := make(chan chainCheckResult, len(chains))

	// 使用 worker pool 限制并发数，避免链索引扩展后产生过多 goroutine
	workerCount := MaxAPIConcurrency
	if workerCount > len(chains) {
		workerCount = len(chains)
	}
	if workerCount < 1 {
		workerCount = 1
	}

	jobs := make(chan ChainConfig, len(chains))
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for chain := range jobs {
				func(chain ChainConfig) {
					defer func() {
						if r := recover(); r != nil {
							log.Debug().Str("chain", chain.Name).Any("panic", r).Msg("检查链时发生异常")
							ch <- chainCheckResult{failed: true, chain: chain}
						}
					}()

					var addr string
					var balance *big.Int
					var checkErr error

					switch chain.Type {
					case "evm":
						addr, checkErr = DeriveEVMAddress(mnemonic, chain.DerivationPath)
						if checkErr != nil {
							log.Debug().Err(checkErr).Str("chain", chain.Name).Msg("地址派生失败")
							ch <- chainCheckResult{failed: true, chain: chain}
							return
						}
						balance, checkErr = CheckEVMBalance(addr, chain.RPCs, chain.ExplorerAPI)

					case "bitcoin":
						addr, checkErr = DeriveBitcoinAddress(mnemonic, chain.DerivationPath)
						if checkErr != nil {
							log.Debug().Err(checkErr).Str("chain", chain.Name).Msg("地址派生失败")
							ch <- chainCheckResult{failed: true, chain: chain}
							return
						}
						balance, checkErr = CheckBitcoinBalance(addr)

					case "solana":
						addr, checkErr = DeriveSolanaAddress(mnemonic, chain.DerivationPath)
						if checkErr != nil {
							log.Debug().Err(checkErr).Str("chain", chain.Name).Msg("地址派生失败")
							ch <- chainCheckResult{failed: true, chain: chain}
							return
						}
						balance, checkErr = CheckSolanaBalance(addr)

					case "cosmos":
						addr, checkErr = DeriveCosmosAddress(mnemonic, chain.DerivationPath, chain.Bech32Prefix)
						if checkErr != nil {
							log.Debug().Err(checkErr).Str("chain", chain.Name).Msg("地址派生失败")
							ch <- chainCheckResult{failed: true, chain: chain}
							return
						}
						balance, checkErr = CheckCosmosBalance(addr, chain.LCD)

					default:
						log.Debug().Str("chain", chain.Name).Str("type", chain.Type).Msg("不支持的链类型")
						ch <- chainCheckResult{failed: true, chain: chain}
						return
					}

					if checkErr != nil {
						log.Debug().Err(checkErr).Str("chain", chain.Name).Str("addr", addr).Msg("余额查询失败")
						ch <- chainCheckResult{failed: true, chain: chain}
						return
					}

					if balance != nil && balance.Cmp(big.NewInt(0)) > 0 {
						ch <- chainCheckResult{
							res: ChainResult{
								ChainName: chain.Name,
								Symbol:    chain.Symbol,
								Balance:   balance,
								Address:   addr,
							},
							ok: true,
						}
						return
					}
					ch <- chainCheckResult{}
				}(chain)
			}
		}()
	}

	for _, c := range chains {
		jobs <- c
	}
	close(jobs)

	wg.Wait()
	close(ch)

	for r := range ch {
		if r.ok {
			results = append(results, r.res)
		}
		if r.failed {
			failedChains = append(failedChains, r.chain)
		}
	}

	return results, failedChains, nil
}
