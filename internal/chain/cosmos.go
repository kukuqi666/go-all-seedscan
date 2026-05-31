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

	"github.com/btcsuite/btcd/btcutil/bech32"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

func DeriveCosmosAddress(mnemonic, pathStr, prefix string) (string, error) {
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
	conv, err := bech32.ConvertBits(compressed, 8, 5, true)
	if err != nil {
		return "", errors.Wrap(err, "bech32 转换失败")
	}
	encoded, err := bech32.Encode(prefix, conv)
	if err != nil {
		return "", errors.Wrap(err, "bech32 编码失败")
	}
	return encoded, nil
}

func CheckCosmosBalance(address, lcdURL string) (*big.Int, error) {
	const maxRetries = 3
	url := fmt.Sprintf("%s/cosmos/bank/v1beta1/balances/%s", lcdURL, address)

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 800 * time.Millisecond)
		}

		acquireAPI()
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			cancel()
			releaseAPI()
			continue
		}

		resp, err := http.DefaultClient.Do(req)
		cancel()
		releaseAPI()

		if err != nil {
			continue
		}

		if resp.StatusCode == 429 {
			resp.Body.Close()
			time.Sleep(3 * time.Second)
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
			Balances []struct {
				Denom  string `json:"denom"`
				Amount string `json:"amount"`
			} `json:"balances"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		for _, b := range result.Balances {
			if strings.HasPrefix(b.Denom, "u") {
				amt, ok := new(big.Int).SetString(b.Amount, 10)
				if ok && amt.Cmp(big.NewInt(0)) > 0 {
					return amt, nil
				}
			}
		}

		return big.NewInt(0), nil
	}

	return nil, fmt.Errorf("查询 Cosmos 余额失败（重试%d次）", maxRetries)
}
