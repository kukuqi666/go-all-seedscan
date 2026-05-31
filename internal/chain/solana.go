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

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/ed25519"
)

func DeriveSolanaAddress(mnemonic, pathStr string) (string, error) {
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

	if len(currentKey.Key) != ed25519.PrivateKeySize {
		privKey := ed25519.NewKeyFromSeed(currentKey.Key)
		pubKey := privKey.Public().(ed25519.PublicKey)
		return base58.Encode(pubKey), nil
	}

	privKey := ed25519.PrivateKey(currentKey.Key)
	pubKey := privKey.Public().(ed25519.PublicKey)
	return base58.Encode(pubKey), nil
}

func CheckSolanaBalance(address string) (*big.Int, error) {
	const maxRetries = 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}

		reqBody := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"getBalance","params":["%s"]}`, address)
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.mainnet-beta.solana.com", strings.NewReader(reqBody))
		if err != nil {
			cancel()
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		cancel()

		if err != nil {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		var result struct {
			Result struct {
				Value int64 `json:"value"`
			} `json:"result"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		if result.Error != nil {
			continue
		}

		return big.NewInt(result.Result.Value), nil
	}

	return nil, fmt.Errorf("查询 Solana 余额失败（重试%d次）", maxRetries)
}
