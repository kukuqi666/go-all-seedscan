package chain

import (
	"fmt"
	"math/big"
	"strings"
	"time"
)

const (
	MaxAPIConcurrency = 32
	APITimeout        = 5 * time.Second
)

type ChainConfig struct {
	Name           string
	Symbol         string
	ShortName      string
	DerivationPath string
	RPC            string
	Type           string
}

type ChainResult struct {
	ChainName string
	Symbol    string
	Balance   *big.Int
	Address   string
}

var Chains = []ChainConfig{
	{
		Name:           "Ethereum",
		Symbol:         "ETH",
		ShortName:      "eth",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://rpc.ankr.com/eth",
		Type:           "evm",
	},
	{
		Name:           "BNB Chain",
		Symbol:         "BNB",
		ShortName:      "bsc",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://rpc.ankr.com/bsc",
		Type:           "evm",
	},
	{
		Name:           "Polygon",
		Symbol:         "MATIC",
		ShortName:      "polygon",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://rpc.ankr.com/polygon",
		Type:           "evm",
	},
	{
		Name:           "Avalanche C-Chain",
		Symbol:         "AVAX",
		ShortName:      "avax",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://rpc.ankr.com/avalanche",
		Type:           "evm",
	},
	{
		Name:           "Arbitrum",
		Symbol:         "ARB",
		ShortName:      "arb",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://rpc.ankr.com/arbitrum",
		Type:           "evm",
	},
	{
		Name:           "Optimism",
		Symbol:         "OP",
		ShortName:      "op",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://rpc.ankr.com/optimism",
		Type:           "evm",
	},
}

func FilterChains(chainSpec string) ([]ChainConfig, error) {
	if chainSpec == "" || strings.EqualFold(chainSpec, "all") {
		return Chains, nil
	}

	shortMap := make(map[string]ChainConfig)
	for _, c := range Chains {
		shortMap[strings.ToLower(c.ShortName)] = c
	}

	var selected []ChainConfig
	for _, name := range strings.Split(chainSpec, ",") {
		name = strings.TrimSpace(strings.ToLower(name))
		if c, ok := shortMap[name]; ok {
			selected = append(selected, c)
		} else {
			return nil, fmt.Errorf("未知的链: %s (可选: eth,bsc,polygon,avax,arb,op)", name)
		}
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("至少需要选择一条链")
	}
	return selected, nil
}
