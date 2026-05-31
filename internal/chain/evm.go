package chain

import (
	"context"
	"fmt"
	"math/big"
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

func CheckEVMBalance(address string, rpcURL string) (*big.Int, error) {
	const maxRetries = 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}

		client, err := getRPCClient(rpcURL)
		if err != nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		balance, err := client.BalanceAt(ctx, common.HexToAddress(address), nil)
		cancel()

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

func CheckMnemonicOnChains(mnemonic string, chains []ChainConfig) (results []ChainResult, failedCount int, err error) {
	type chainCheckResult struct {
		res    ChainResult
		ok     bool
		failed bool
	}

	ch := make(chan chainCheckResult, len(chains))
	for _, c := range chains {
		go func(chain ChainConfig) {
			defer func() {
				if r := recover(); r != nil {
					log.Debug().Str("chain", chain.Name).Any("panic", r).Msg("检查链时发生异常")
					ch <- chainCheckResult{failed: true}
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
					ch <- chainCheckResult{failed: true}
					return
				}
				balance, checkErr = CheckEVMBalance(addr, chain.RPC)

			case "bitcoin":
				addr, checkErr = DeriveBitcoinAddress(mnemonic, chain.DerivationPath)
				if checkErr != nil {
					log.Debug().Err(checkErr).Str("chain", chain.Name).Msg("地址派生失败")
					ch <- chainCheckResult{failed: true}
					return
				}
				balance, checkErr = CheckBitcoinBalance(addr)

			case "solana":
				addr, checkErr = DeriveSolanaAddress(mnemonic, chain.DerivationPath)
				if checkErr != nil {
					log.Debug().Err(checkErr).Str("chain", chain.Name).Msg("地址派生失败")
					ch <- chainCheckResult{failed: true}
					return
				}
				balance, checkErr = CheckSolanaBalance(addr)

			case "cosmos":
				addr, checkErr = DeriveCosmosAddress(mnemonic, chain.DerivationPath, chain.Bech32Prefix)
				if checkErr != nil {
					log.Debug().Err(checkErr).Str("chain", chain.Name).Msg("地址派生失败")
					ch <- chainCheckResult{failed: true}
					return
				}
				balance, checkErr = CheckCosmosBalance(addr, chain.LCD)

			default:
				log.Debug().Str("chain", chain.Name).Str("type", chain.Type).Msg("不支持的链类型")
				ch <- chainCheckResult{failed: true}
				return
			}

			if checkErr != nil {
				log.Debug().Err(checkErr).Str("chain", chain.Name).Str("addr", addr).Msg("余额查询失败")
				ch <- chainCheckResult{failed: true}
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
		}(c)
	}

	for range chains {
		r := <-ch
		if r.ok {
			results = append(results, r.res)
		}
		if r.failed {
			failedCount++
		}
	}

	return results, failedCount, nil
}
