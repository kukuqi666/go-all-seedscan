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
	Chains         string
}

func Parse() Config {
	var cfg Config
	pflag.StringVarP(&cfg.SeedPhrase, "seed", "s", "", "助记词模板，缺失单词用 ? 表示")
	pflag.IntVarP(&cfg.MaxWordMissing, "max-word", "m", 5, "允许缺失的最大单词数量")
	pflag.IntVarP(&cfg.Workers, "workers", "w", runtime.NumCPU(), "并发 worker 数量")
	pflag.BoolVarP(&cfg.Debug, "debug", "d", false, "开启调试日志")
	pflag.BoolVarP(&cfg.StopOnFirst, "stop-first", "f", true, "找到第一个有余额的钱包后停止")
	pflag.StringVarP(&cfg.Chains, "chains", "c", "all", "要扫描的链，逗号分隔 (eth,bsc,polygon,avax,arb,op) 或 all")
	pflag.Parse()
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
