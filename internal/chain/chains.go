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
	Bech32Prefix   string
	LCD            string
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
		Symbol:         "ETH",
		ShortName:      "arb",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://rpc.ankr.com/arbitrum",
		Type:           "evm",
	},
	{
		Name:           "Optimism",
		Symbol:         "ETH",
		ShortName:      "op",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://rpc.ankr.com/optimism",
		Type:           "evm",
	},
	{
		Name:           "Base",
		Symbol:         "ETH",
		ShortName:      "base",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://rpc.ankr.com/base",
		Type:           "evm",
	},
	{
		Name:           "zkSync Era",
		Symbol:         "ETH",
		ShortName:      "zksync",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://mainnet.era.zksync.io",
		Type:           "evm",
	},
	{
		Name:           "Linea",
		Symbol:         "ETH",
		ShortName:      "linea",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://rpc.ankr.com/linea",
		Type:           "evm",
	},
	{
		Name:           "Mantle",
		Symbol:         "MNT",
		ShortName:      "mantle",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://rpc.ankr.com/mantle",
		Type:           "evm",
	},
	{
		Name:           "Scroll",
		Symbol:         "ETH",
		ShortName:      "scroll",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://rpc.ankr.com/scroll",
		Type:           "evm",
	},
	{
		Name:           "Fantom",
		Symbol:         "FTM",
		ShortName:      "ftm",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://rpc.ankr.com/fantom",
		Type:           "evm",
	},
	{
		Name:           "Celo",
		Symbol:         "CELO",
		ShortName:      "celo",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://forno.celo.org",
		Type:           "evm",
	},
	{
		Name:           "Cronos",
		Symbol:         "CRO",
		ShortName:      "cro",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPC:            "https://evm.cronos.org",
		Type:           "evm",
	},
	{
		Name:           "Bitcoin (Native SegWit)",
		Symbol:         "BTC",
		ShortName:      "btc",
		DerivationPath: "m/84'/0'/0'/0/0",
		Type:           "bitcoin",
	},
	{
		Name:           "Bitcoin (Nested SegWit)",
		Symbol:         "BTC",
		ShortName:      "btc-p2sh",
		DerivationPath: "m/49'/0'/0'/0/0",
		Type:           "bitcoin",
	},
	{
		Name:           "Bitcoin (Legacy)",
		Symbol:         "BTC",
		ShortName:      "btc-legacy",
		DerivationPath: "m/44'/0'/0'/0/0",
		Type:           "bitcoin",
	},
	{
		Name:           "Solana",
		Symbol:         "SOL",
		ShortName:      "sol",
		DerivationPath: "m/44'/501'/0'/0'",
		Type:           "solana",
	},
	{
		Name:           "Cosmos (ATOM)",
		Symbol:         "ATOM",
		ShortName:      "atom",
		DerivationPath: "m/44'/118'/0'/0/0",
		Type:           "cosmos",
		Bech32Prefix:   "cosmos",
		LCD:            "https://lcd.cosmoshub.bigorbust.io",
	},
	{
		Name:           "Osmosis",
		Symbol:         "OSMO",
		ShortName:      "osmo",
		DerivationPath: "m/44'/118'/0'/0/0",
		Type:           "cosmos",
		Bech32Prefix:   "osmo",
		LCD:            "https://lcd.osmosis.zone",
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
			return nil, fmt.Errorf("未知的链: %s (可选: eth,bsc,polygon,avax,arb,op,base,zksync,linea,mantle,scroll,ftm,celo,cro,btc,btc-p2sh,btc-legacy,sol,atom,osmo)", name)
		}
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("至少需要选择一条链")
	}
	return selected, nil
}
