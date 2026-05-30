package main

import (
	"time"

	"recover/internal/config"
	"recover/internal/core"
	"recover/internal/logger"
	"recover/internal/ui"

	"github.com/rs/zerolog/log"
)

func main() {
	cfg := config.Parse()
	logger.Init(cfg.Debug)
	if err := config.Validate(cfg); err != nil {
		log.Fatal().Err(err).Msg("配置无效")
	}

	core.InitWordList()

	dashboard := ui.NewDashboardUI()
	defer dashboard.Close()

	start := time.Now()
	core.RecoverSeedPhrase(cfg, dashboard)
	dashboard.PrintSummary(time.Since(start))
}
