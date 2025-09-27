package internal

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"
)

// DisplaySingleRun 以美观、可视化的格式显示单次运行的结果。
// r: 单次测试的结果。
// testType: 当前执行的测试类型 ("all", "dns", "tcp")。
func DisplaySingleRun(r TraceResult, testType string) {
	color := TimingColor(r.Total, 200*time.Millisecond, 1*time.Second)
	fmt.Printf("%s✓ %s分析%s\n", ColorGreen, strings.ToUpper(testType), ColorReset)

	totalNanos := float64(r.Total.Nanoseconds())
	if totalNanos == 0 {
		totalNanos = 1 // 防止在总耗时为0时出现除以零的错误
	}

	// printBar 是一个内部辅助函数，用于打印带可视化条形图的单行结果。
	printBar := func(name string, d time.Duration) {
		if d > 0 {
			// 计算该阶段耗时占总耗时的比例，并据此生成条形图
			ratio := float64(d.Nanoseconds()) / totalNanos
			barWidth := int(math.Ceil(ratio * 20)) // 20是条形图的最大宽度字符数
			bar := strings.Repeat("█", barWidth)
			c := TimingColor(d, 100*time.Millisecond, 500*time.Millisecond)
			fmt.Printf("├─ %-15s [%-20s] %s%8.2f ms%s\n", name, bar, c, float64(d.Milliseconds()), ColorReset)
		}
	}

	// 根据测试类型，打印不同的结果项
	switch testType {
	case "all":
		printBar("DNS 查询", r.DNSLookup)
		printBar("TCP 连接", r.TCPConnection)
		if r.TLSHandshake > 0 { // 仅当存在TLS握手时才显示
			printBar("TLS 握手", r.TLSHandshake)
		}
		printBar("服务器处理 (TTFB)", r.ServerProcessing)
		printBar("内容传输", r.ContentTransfer)
	case "dns":
		fmt.Printf("├─ %-15s: %s%.2f ms%s\n", "解析耗时", color, float64(r.DNSLookup.Milliseconds()), ColorReset)
	case "tcp":
		fmt.Printf("├─ %-1s: %s%.2f ms%s\n", "连接耗时", color, float64(r.TCPConnection.Milliseconds()), ColorReset)
	}

	fmt.Printf("\n%s✓ %s总体统计%s\n", ColorGreen, strings.ToUpper(testType), ColorReset)
	fmt.Printf("├─ %-15s: %s%.2f ms%s\n", "总耗时", color, float64(r.Total.Milliseconds()), ColorReset)
	if len(r.ResolvedIPs) > 0 {
		fmt.Printf("└─ %-15s: %s\n", "解析IP", strings.Join(r.ResolvedIPs, ", "))
	}
}

// DisplaySummary 以摘要的形式显示多次运行的统计结果。
// results: 包含所有测试运行结果的切片。
// testType: 当前执行的测试类型。
func DisplaySummary(results []TraceResult, testType string) {
	fmt.Printf("%s✓ 测试结果汇总 (共 %d 次运行)%s\n\n", ColorGreen, len(results), ColorReset)

	// printStats 是一个内部辅助函数，用于计算并打印单个指标的统计数据。
	printStats := func(name string, times []time.Duration) {
		if len(times) < 1 {
			return // 如果没有数据则不打印
		}
		stats := CalculateStats(times)
		c := TimingColor(stats.Avg, 100*time.Millisecond, 500*time.Millisecond)

		var runsMs []string
		for _, r := range stats.AllRuns {
			runsMs = append(runsMs, fmt.Sprintf("%.2f", float64(r.Milliseconds())))
		}

		fmt.Printf("%s%s:%s\n", ColorBold, name, ColorReset)
		fmt.Printf("├─ %-10s: [%s] ms\n", "耗时列表", strings.Join(runsMs, ", "))
		fmt.Printf("└─ %-10s: %s平均 %.2f ms, 最快 %.2f ms, 最慢 %.2f ms%s\n\n", "统计", c,
			float64(stats.Avg.Milliseconds()),
			float64(stats.Min.Milliseconds()),
			float64(stats.Max.Milliseconds()),
			ColorReset)
	}

	// 根据测试类型，提取并打印每个指标的统计数据
	if testType == "all" || testType == "dns" {
		var dnsTimes []time.Duration
		for _, r := range results {
			dnsTimes = append(dnsTimes, r.DNSLookup)
		}
		printStats("DNS 查询", dnsTimes)
	}
	if testType == "all" || testType == "tcp" {
		var tcpTimes []time.Duration
		for _, r := range results {
			tcpTimes = append(tcpTimes, r.TCPConnection)
		}
		printStats("TCP 连接", tcpTimes)
	}
	if testType == "all" {
		var tlsTimes []time.Duration
		for _, r := range results {
			if r.TLSHandshake > 0 {
				tlsTimes = append(tlsTimes, r.TLSHandshake)
			}
		}
		printStats("TLS 握手", tlsTimes)

		var serverTimes []time.Duration
		for _, r := range results {
			serverTimes = append(serverTimes, r.ServerProcessing)
		}
		printStats("服务器处理 (TTFB)", serverTimes)

		var transferTimes []time.Duration
		for _, r := range results {
			transferTimes = append(transferTimes, r.ContentTransfer)
		}
		printStats("内容传输", transferTimes)

		var totalTimes []time.Duration
		for _, r := range results {
			totalTimes = append(totalTimes, r.Total)
		}
		printStats("总耗时", totalTimes)
	}
}

// TimingColor 根据耗时返回不同的颜色，用于在视觉上区分响应速度。
// d: 耗时。
// good: 低于此时长为“快”，显示绿色。
// bad: 高于此时长为“慢”，显示红色。
func TimingColor(d, good, bad time.Duration) string {
	if d < good {
		return ColorGreen
	}
	if d < bad {
		return ColorYellow
	}
	return ColorRed
}

// PrintError 以标准错误格式打印红色的错误信息。
func PrintError(msg string) {
	_, _ = fmt.Fprintf(os.Stderr, "%s✗ 错误: %s%s\n", ColorRed, msg, ColorReset)
}
