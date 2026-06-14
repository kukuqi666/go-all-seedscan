package config

import (
	"fmt"
	"runtime"

	"github.com/spf13/pflag"
)

type Config struct {
	SeedPhrase     string
	MaxWordMissing int
	Workers        int
	Debug          bool
	StopOnFirst    bool
	ShowAll        bool
	Chains         string
	TargetAddress  string
	Offline        bool
	APIKey         string
	CustomRPC      string
	MaxAddrIndex   int
}

func Parse() Config {
	var cfg Config
	pflag.StringVarP(&cfg.SeedPhrase, "seed", "s", "", "助记词模板，缺失单词用 ? 表示")
	pflag.IntVarP(&cfg.MaxWordMissing, "max-word", "m", 5, "允许缺失的最大单词数量")
	pflag.IntVarP(&cfg.Workers, "workers", "w", runtime.NumCPU(), "并发 worker 数量")
	pflag.BoolVarP(&cfg.Debug, "debug", "d", false, "开启调试日志")
	pflag.BoolVarP(&cfg.StopOnFirst, "stop-first", "f", true, "找到第一个有余额的钱包后停止")
	pflag.BoolVar(&cfg.ShowAll, "show-all", false, "扫描结束后列出所有有效助记词（无余额且无目标地址时用于手动验证）")
	pflag.StringVarP(&cfg.Chains, "chains", "c", "all", "要扫描的链，逗号分隔 (eth,bsc,polygon,avax,arb,op,base,zksync,linea,mantle,scroll,ftm,celo,cro,btc,btc-p2sh,btc-legacy,sol,atom,osmo) 或 all")
	pflag.StringVarP(&cfg.TargetAddress, "address", "a", "", "目标钱包地址（离线模式：本地比对地址，无需API）多个地址用逗号分隔")
	pflag.StringVarP(&cfg.APIKey, "api-key", "k", "", "区块浏览器 API Key（BscScan/Etherscan 等，大幅提升稳定性）")
	pflag.StringVarP(&cfg.CustomRPC, "rpc", "r", "", "自定义 RPC 端点（覆盖默认端点，支持付费RPC）")
	pflag.IntVarP(&cfg.MaxAddrIndex, "addr-index", "i", 4, "检查的地址索引范围 0~N（不同钱包可能用不同索引）")
	pflag.Parse()

	cfg.Offline = cfg.TargetAddress != ""
	return cfg
}

func Validate(cfg Config) error {
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
