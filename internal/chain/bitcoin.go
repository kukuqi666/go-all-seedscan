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
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/bech32"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

var bitcoinNetParams = &chaincfg.MainNetParams

func DeriveBitcoinAddress(mnemonic, pathStr string) (string, error) {
	seed := bip39.NewSeed(mnemonic, "")
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return "", errors.Wrap(err, "生成主密钥失败")
	}

	currentKey := masterKey
	parts := strings.Split(pathStr, "/")
	if len(parts) < 2 || parts[0] != "m" {
		return "", fmt.Errorf("无效的路径格式: %s", pathStr)
	}

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

	privKey, err := crypto.ToECDSA(currentKey.Key)
	if err != nil {
		return "", errors.Wrap(err, "转换为 ECDSA 私钥失败")
	}

	compressed := crypto.CompressPubkey(&privKey.PublicKey)

	purpose := parts[1]
	if strings.HasSuffix(purpose, "'") {
		purpose = purpose[:len(purpose)-1]
	}

	switch purpose {
	case "84":
		pubKeyHash := btcutil.Hash160(compressed)
		conv, err := bech32.ConvertBits(pubKeyHash, 8, 5, true)
		if err != nil {
			return "", errors.Wrap(err, "bech32 转换失败")
		}
		encoded, err := bech32.Encode("bc", conv)
		if err != nil {
			return "", errors.Wrap(err, "bech32 编码失败")
		}
		return encoded, nil

	case "49":
		pubKeyHash := btcutil.Hash160(compressed)
		scriptSig, err := txscript.NewScriptBuilder().
			AddOp(txscript.OP_0).
			AddData(pubKeyHash).
			Script()
		if err != nil {
			return "", errors.Wrap(err, "构建脚本失败")
		}
		scriptHash := btcutil.Hash160(scriptSig)
		addr, err := btcutil.NewAddressScriptHashFromHash(scriptHash, bitcoinNetParams)
		if err != nil {
			return "", errors.Wrap(err, "创建 P2SH 地址失败")
		}
		return addr.EncodeAddress(), nil

	default:
		pubKey := crypto.FromECDSAPub(&privKey.PublicKey)
		pubKeyHash := btcutil.Hash160(pubKey)
		addr, err := btcutil.NewAddressPubKeyHash(pubKeyHash, bitcoinNetParams)
		if err != nil {
			return "", errors.Wrap(err, "创建 P2PKH 地址失败")
		}
		return addr.EncodeAddress(), nil
	}
}

func CheckBitcoinBalance(address string) (*big.Int, error) {
	const maxRetries = 3
	url := fmt.Sprintf("https://blockstream.info/api/address/%s", address)

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}

		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			cancel()
			continue
		}
		req.Header.Set("User-Agent", "seedscan/1.0")

		resp, err := http.DefaultClient.Do(req)
		cancel()

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
			ChainStats struct {
				FundedTxoSum int64 `json:"funded_txo_sum"`
				SpentTxoSum  int64 `json:"spent_txo_sum"`
			} `json:"chain_stats"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		balance := result.ChainStats.FundedTxoSum - result.ChainStats.SpentTxoSum
		return big.NewInt(balance), nil
	}

	return nil, fmt.Errorf("查询 Bitcoin 余额失败（重试%d次）", maxRetries)
}
