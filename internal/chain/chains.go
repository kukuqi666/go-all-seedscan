package chain

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

const (
	MaxAPIConcurrency = 3
	APITimeout        = 8 * time.Second
	MinAPIDelay       = 300 * time.Millisecond
)

var apiLimiter = make(chan struct{}, MaxAPIConcurrency)

func init() {
	for i := 0; i < MaxAPIConcurrency; i++ {
		apiLimiter <- struct{}{}
	}
}

func acquireAPI() {
	<-apiLimiter
}

func releaseAPI() {
	apiLimiter <- struct{}{}
}

type ChainConfig struct {
	Name           string
	Symbol         string
	ShortName      string
	DerivationPath string
	RPCs           []string
	Type           string
	Bech32Prefix   string
	LCD            string
	ExplorerAPI    string
}

func (c ChainConfig) RPC() string {
	if len(c.RPCs) > 0 {
		return c.RPCs[0]
	}
	return ""
}

type ChainResult struct {
	ChainName string
	Symbol    string
	Balance   *big.Int
	Address   string
}

type AddressMatch struct {
	ChainName string
	Symbol    string
	Address   string
}

func DeriveMnemonicAddresses(mnemonic string, chains []ChainConfig) []AddressMatch {
	var matches []AddressMatch

	for _, c := range chains {
		var addr string
		var err error

		switch c.Type {
		case "evm":
			addr, err = DeriveEVMAddress(mnemonic, c.DerivationPath)
		case "bitcoin":
			addr, err = DeriveBitcoinAddress(mnemonic, c.DerivationPath)
		case "solana":
			addr, err = DeriveSolanaAddress(mnemonic, c.DerivationPath)
		case "cosmos":
			addr, err = DeriveCosmosAddress(mnemonic, c.DerivationPath, c.Bech32Prefix)
		default:
			continue
		}

		if err != nil {
			continue
		}

		matches = append(matches, AddressMatch{
			ChainName: c.Name,
			Symbol:    c.Symbol,
			Address:   addr,
		})
	}

	return matches
}

var Chains = []ChainConfig{
	{
		Name: "Ethereum", Symbol: "ETH", ShortName: "eth",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPCs: []string{
			"https://rpc.ankr.com/eth",
			"https://ethereum-rpc.publicnode.com",
			"https://cloudflare-eth.com",
		},
		Type:        "evm",
		ExplorerAPI: "https://api.etherscan.io/api",
	},
	{
		Name: "BNB Chain", Symbol: "BNB", ShortName: "bsc",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPCs: []string{
			"https://rpc.ankr.com/bsc",
			"https://bsc-rpc.publicnode.com",
			"https://bsc-dataseed.binance.org",
		},
		Type:        "evm",
		ExplorerAPI: "https://api.bscscan.com/api",
	},
	{
		Name: "Polygon", Symbol: "MATIC", ShortName: "polygon",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPCs: []string{
			"https://rpc.ankr.com/polygon",
			"https://polygon-bor-rpc.publicnode.com",
		},
		Type:        "evm",
		ExplorerAPI: "https://api.polygonscan.com/api",
	},
	{
		Name: "Avalanche C-Chain", Symbol: "AVAX", ShortName: "avax",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPCs: []string{
			"https://rpc.ankr.com/avalanche",
			"https://avalanche-c-chain-rpc.publicnode.com",
		},
		Type:        "evm",
		ExplorerAPI: "https://api.snowtrace.io/api",
	},
	{
		Name: "Arbitrum", Symbol: "ETH", ShortName: "arb",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPCs: []string{
			"https://rpc.ankr.com/arbitrum",
			"https://arbitrum-one-rpc.publicnode.com",
		},
		Type:        "evm",
		ExplorerAPI: "https://api.arbiscan.io/api",
	},
	{
		Name: "Optimism", Symbol: "ETH", ShortName: "op",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPCs: []string{
			"https://rpc.ankr.com/optimism",
			"https://optimism-rpc.publicnode.com",
		},
		Type:        "evm",
		ExplorerAPI: "https://api-optimistic.etherscan.io/api",
	},
	{
		Name: "Base", Symbol: "ETH", ShortName: "base",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPCs: []string{
			"https://rpc.ankr.com/base",
			"https://base-rpc.publicnode.com",
		},
		Type:        "evm",
		ExplorerAPI: "https://api.basescan.org/api",
	},
	{
		Name: "zkSync Era", Symbol: "ETH", ShortName: "zksync",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPCs:           []string{"https://mainnet.era.zksync.io"},
		Type:           "evm",
	},
	{
		Name: "Linea", Symbol: "ETH", ShortName: "linea",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPCs: []string{
			"https://rpc.ankr.com/linea",
			"https://linea-rpc.publicnode.com",
		},
		Type:        "evm",
		ExplorerAPI: "https://api.lineascan.build/api",
	},
	{
		Name: "Mantle", Symbol: "MNT", ShortName: "mantle",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPCs:           []string{"https://rpc.ankr.com/mantle"},
		Type:           "evm",
	},
	{
		Name: "Scroll", Symbol: "ETH", ShortName: "scroll",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPCs:           []string{"https://rpc.ankr.com/scroll"},
		Type:           "evm",
	},
	{
		Name: "Fantom", Symbol: "FTM", ShortName: "ftm",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPCs: []string{
			"https://rpc.ankr.com/fantom",
			"https://fantom-rpc.publicnode.com",
		},
		Type:        "evm",
		ExplorerAPI: "https://api.ftmscan.com/api",
	},
	{
		Name: "Celo", Symbol: "CELO", ShortName: "celo",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPCs:           []string{"https://forno.celo.org"},
		Type:           "evm",
	},
	{
		Name: "Cronos", Symbol: "CRO", ShortName: "cro",
		DerivationPath: "m/44'/60'/0'/0/0",
		RPCs:           []string{"https://evm.cronos.org"},
		Type:           "evm",
	},
	{
		Name: "Bitcoin (Native SegWit)", Symbol: "BTC", ShortName: "btc",
		DerivationPath: "m/84'/0'/0'/0/0",
		Type:           "bitcoin",
	},
	{
		Name: "Bitcoin (Nested SegWit)", Symbol: "BTC", ShortName: "btc-p2sh",
		DerivationPath: "m/49'/0'/0'/0/0",
		Type:           "bitcoin",
	},
	{
		Name: "Bitcoin (Legacy)", Symbol: "BTC", ShortName: "btc-legacy",
		DerivationPath: "m/44'/0'/0'/0/0",
		Type:           "bitcoin",
	},
	{
		Name: "Solana", Symbol: "SOL", ShortName: "sol",
		DerivationPath: "m/44'/501'/0'/0'",
		Type:           "solana",
	},
	{
		Name: "Cosmos (ATOM)", Symbol: "ATOM", ShortName: "atom",
		DerivationPath: "m/44'/118'/0'/0/0",
		Type:           "cosmos", Bech32Prefix: "cosmos",
		LCD: "https://lcd.cosmoshub.bigorbust.io",
	},
	{
		Name: "Osmosis", Symbol: "OSMO", ShortName: "osmo",
		DerivationPath: "m/44'/118'/0'/0/0",
		Type:           "cosmos", Bech32Prefix: "osmo",
		LCD: "https://lcd.osmosis.zone",
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

func ApplyCustomRPC(chains []ChainConfig, rpcURL string) []ChainConfig {
	if rpcURL == "" {
		return chains
	}

	result := make([]ChainConfig, len(chains))
	for i, c := range chains {
		result[i] = c
		if c.Type == "evm" {
			result[i].RPCs = []string{rpcURL}
		}
	}
	return result
}

func ExpandChainIndices(chains []ChainConfig, maxIndex int) []ChainConfig {
	if maxIndex <= 0 {
		return chains
	}

	var expanded []ChainConfig
	for _, c := range chains {
		switch c.Type {
		case "evm":
			base := "m/44'/60'/0'/0/"
			for idx := 0; idx <= maxIndex; idx++ {
				cc := c
				cc.DerivationPath = base + strconv.Itoa(idx)
				if idx > 0 {
					cc.Name = fmt.Sprintf("%s (索引%d)", c.Name, idx)
				}
				expanded = append(expanded, cc)
			}
		case "bitcoin":
			for idx := 0; idx <= maxIndex; idx++ {
				cc := c
				parts := strings.Split(c.DerivationPath, "/")
				parts[len(parts)-1] = strconv.Itoa(idx)
				cc.DerivationPath = strings.Join(parts, "/")
				if idx > 0 {
					cc.Name = fmt.Sprintf("%s (索引%d)", c.Name, idx)
				}
				expanded = append(expanded, cc)
			}
		default:
			expanded = append(expanded, c)
		}
	}
	return expanded
}
