package ui

import (
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	colorReset   = "\x1b[0m"
	colorBold    = "\x1b[1m"
	colorDim     = "\x1b[2m"
	colorCyan    = "\x1b[36m"
	colorBlue    = "\x1b[34m"
	colorGreen   = "\x1b[32m"
	colorYellow  = "\x1b[33m"
	colorRed     = "\x1b[31m"
	colorMagenta = "\x1b[35m"
	PanelWidth   = 80
)

type DashboardUI struct {
	mu      sync.Mutex
	started bool
	closed  bool
}

func NewDashboardUI() *DashboardUI {
	return &DashboardUI{}
}

func (d *DashboardUI) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	d.closed = true
	fmt.Fprintln(os.Stdout)
}

func (d *DashboardUI) StartSession(missingWords int, searchSpace *big.Int, workers int, stopOnFirst bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.started = true

	printBanner()
	printPanel("会话信息",
		fmt.Sprintf("模式         : %sBIP39 助记词恢复 + 多链 EVM 余额扫描%s", colorBlue, colorReset),
		fmt.Sprintf("缺失单词数   : %d", missingWords),
		fmt.Sprintf("并发数       : %d", workers),
		fmt.Sprintf("搜索空间     : %s", formatBigInt(searchSpace)),
		fmt.Sprintf("找到即停止   : %t", stopOnFirst),
	)
	fmt.Fprintln(os.Stdout)
}

func (d *DashboardUI) UpdateProgress(processedBranches, totalBranches uint64, estimatedProcessed, totalComb *big.Int, validMnemonics, checkedMnemonics, hitsFound uint64, elapsed time.Duration, status string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.started || d.closed {
		return
	}

	pct := float64(processedBranches) / float64(max(totalBranches, 1)) * 100
	rate := float64(checkedMnemonics) / max(elapsed.Seconds(), 0.001)

	var line string
	if pct >= 100 && validMnemonics > 0 {
		checkPct := float64(checkedMnemonics) / float64(validMnemonics) * 100
		bar := renderBar(checkPct, 20)
		remaining := ""
		if checkPct > 0 && checkPct < 100 {
			remainingSec := elapsed.Seconds() * (100 - checkPct) / checkPct
			remaining = "~" + formatDurationShort(time.Duration(remainingSec*float64(time.Second)))
		}
		line = fmt.Sprintf("%s 枚举完成 检查%s%s/%s%s %s%5.1f%%%s 已用%s 预计%s 速度%s%.0f条/s%s 命中%s",
			bar,
			colorYellow, formatUint64(checkedMnemonics), formatUint64(validMnemonics), colorReset,
			colorYellow, checkPct, colorReset,
			formatDurationShort(elapsed),
			remaining,
			colorCyan, rate, colorReset,
			formatUint64(hitsFound),
		)
	} else {
		bar := renderBar(pct, 20)
		remaining := ""
		if pct > 0 && pct < 100 {
			remainingSec := elapsed.Seconds() * (100 - pct) / pct
			remaining = "~" + formatDurationShort(time.Duration(remainingSec*float64(time.Second)))
		}
		line = fmt.Sprintf("%s %s%5.1f%%%s 已用%s 预计%s 速度%s%.0f条/s%s 命中%s",
			bar,
			colorYellow, pct, colorReset,
			formatDurationShort(elapsed),
			remaining,
			colorCyan, rate, colorReset,
			formatUint64(hitsFound),
		)
	}

	fmt.Fprintf(os.Stdout, "\r%s\x1b[K", line)
}

func (d *DashboardUI) AddWalletHit(chainName, symbol, address, balance, seed string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	fmt.Fprintln(os.Stdout)
	printPanel(colorGreen+colorBold+"发现钱包"+colorReset,
		fmt.Sprintf("链       : %s (%s)", chainName, symbol),
		fmt.Sprintf("地址     : %s", address),
		fmt.Sprintf("余额     : %s", balance),
		fmt.Sprintf("助记词   : %s", seed),
	)
	fmt.Fprintln(os.Stdout)
}

func (d *DashboardUI) SetMessage(message string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "%s%s%s\n", colorMagenta+colorBold, message, colorReset)
}

func (d *DashboardUI) PrintValidation(valid bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if valid {
		fmt.Fprintf(os.Stdout, "%s✅ 助记词有效%s\n", colorGreen, colorReset)
	} else {
		fmt.Fprintf(os.Stdout, "%s❌ 助记词无效%s\n", colorRed, colorReset)
	}
}

func (d *DashboardUI) PrintSummary(elapsed time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "%s扫描完成，总耗时 %s%s\n", colorCyan+colorBold, formatDurationShort(elapsed), colorReset)
}

func printBanner() {
	fmt.Fprintln(os.Stdout, colorCyan+colorBold+`  ____  ___         _    _     _       ____`+colorReset)
	fmt.Fprintln(os.Stdout, colorCyan+colorBold+` / ___|/ _ \       / \  | |   | |     / ___|  ___  __ _ _ __`+colorReset)
	fmt.Fprintln(os.Stdout, colorCyan+colorBold+`| |  _| | | |     / _ \ | |   | |_____\___ \ / __|/ _`+"`"+` | '_ \`+colorReset)
	fmt.Fprintln(os.Stdout, colorCyan+colorBold+`| |_| | |_| |    / ___ \| |___| |_____|___) | (__| (_| | | | |`+colorReset)
	fmt.Fprintln(os.Stdout, colorCyan+colorBold+` \____|\___/    /_/   \_\_____|_|     |____/ \___|\__,_|_| |_|`+colorReset)
	fmt.Fprintln(os.Stdout)
}

func printPanel(title string, lines ...string) {
	fmt.Fprintln(os.Stdout, panelTitle(title))
	for _, line := range lines {
		fmt.Fprintln(os.Stdout, panelLine(line))
	}
	fmt.Fprintln(os.Stdout, panelBorder())
}

func renderBar(percent float64, width int) string {
	if width <= 0 {
		width = 20
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}

	return colorGreen + "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + colorGreen + "]" + colorReset
}

func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inEscape {
			if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
				inEscape = false
			}
			continue
		}
		if ch == 0x1b {
			inEscape = true
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func panelBorder() string {
	return colorDim + "+" + strings.Repeat("-", PanelWidth-2) + "+" + colorReset
}

func panelTitle(title string) string {
	plainTitle := stripANSI(title)
	fill := PanelWidth - 2 - len(plainTitle) - 2
	if fill < 0 {
		fill = 0
	}
	left := fill / 2
	right := fill - left
	return colorDim + "+" + strings.Repeat("-", left) + colorReset + " " + title + " " + colorDim + strings.Repeat("-", right) + "+" + colorReset
}

func panelLine(content string) string {
	plain := stripANSI(content)
	padding := PanelWidth - 4 - len(plain)
	if padding < 0 {
		padding = 0
	}
	return colorDim + "| " + colorReset + content + strings.Repeat(" ", padding) + colorDim + " |" + colorReset
}

func formatBigInt(n *big.Int) string {
	if n == nil {
		return "0"
	}
	return formatNumberString(n.String())
}

func formatNumberString(s string) string {
	if len(s) <= 3 {
		return s
	}

	sign := ""
	if strings.HasPrefix(s, "-") {
		sign = "-"
		s = s[1:]
	}

	rem := len(s) % 3
	if rem == 0 {
		rem = 3
	}

	var b strings.Builder
	b.Grow(len(s) + len(s)/3)
	b.WriteString(sign)
	b.WriteString(s[:rem])
	for i := rem; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func formatUint64(v uint64) string {
	return formatNumberString(fmt.Sprintf("%d", v))
}

func formatDurationShort(d time.Duration) string {
	if d < time.Second {
		return d.Truncate(time.Millisecond).String()
	}
	return d.Truncate(time.Second).String()
}
