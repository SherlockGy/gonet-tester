package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

// ANSI Color Codes for colorful output.
// 用于彩色输出的ANSI颜色代码
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
	ColorBold   = "\033[1m"
)

// TraceResult 存放单次跟踪的所有计时结果
type TraceResult struct {
	DNSLookup        time.Duration // DNS查询耗时
	TCPConnection    time.Duration // TCP连接耗时
	TLSHandshake     time.Duration // TLS握手耗时
	ServerProcessing time.Duration // 服务器处理耗时 (TTFB)
	ContentTransfer  time.Duration // 内容传输耗时
	Total            time.Duration // 总耗时
	ResolvedIPs      []string      // 解析出的IP地址
}

// Statistics 存放多次运行结果的统计数据
type Statistics struct {
	Min     time.Duration   // 最快耗时
	Max     time.Duration   // 最慢耗时
	Avg     time.Duration   // 平均耗时
	AllRuns []time.Duration // 所有运行的耗时记录
}

func main() {
	// --- 1. 参数解析 ---
	testType := flag.String("test", "all", "要运行的测试类型 (all, dns, tcp)")
	runCount := flag.Int("n", 1, "测试运行的次数")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s用法: gonet-tester [参数] <url>%s\n", ColorBold, ColorReset)
		fmt.Fprintf(os.Stderr, "示例: gonet-tester -n 5 -test=dns google.com\n\n")
		fmt.Fprintf(os.Stderr, "参数:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if len(flag.Args()) != 1 {
		flag.Usage()
		os.Exit(1)
	}

	// --- 2. URL处理 ---
	parsedURL, err := normalizeURL(flag.Args()[0])
	if err != nil {
		printError(err.Error())
		os.Exit(1)
	}

	fmt.Printf("\n%s正在对 %s 进行 %d 次 %s 测试...%s\n\n", ColorBold, parsedURL.String(), *runCount, *testType, ColorReset)

	// --- 3. 执行测试循环 ---
	var results []TraceResult
	// 优化：为所有HTTP测试重复使用同一个client，提高效率
	client := createHttpClient()

	for i := 0; i < *runCount; i++ {
		var result TraceResult
		var err error

		switch *testType {
		case "dns":
			result, err = performDnsTest(parsedURL)
		case "tcp":
			result, err = performTcpTest(parsedURL)
		case "all":
			result, err = performFullTrace(parsedURL, client)
		default:
			printError(fmt.Sprintf("无效的测试类型: %s. 合法值是: all, dns, tcp.", *testType))
			os.Exit(1)
		}

		if err != nil {
			printError(fmt.Sprintf("第 %d 次测试失败: %v", i+1, err))
			// 如果是完全失败，则跳过此次结果
			if result.Total == 0 {
				continue
			}
		}
		results = append(results, result)
	}

	if len(results) == 0 {
		printError("所有测试均失败。 ולאחר מכן נצא מהתוכנית.")
		os.Exit(1)
	}

	// --- 4. 显示结果 ---
	if *runCount == 1 {
		displaySingleRun(results[0], *testType)
	} else {
		displaySummary(results, *testType)
	}
}

// normalizeURL 规范化用户输入的URL，如果缺少协议头则自动添加https
func normalizeURL(rawURL string) (*url.URL, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Host == "" {
		return nil, fmt.Errorf("无效的URL: %s", rawURL)
	}
	return parsedURL, nil
}

// createHttpClient 创建并配置一个可复用的http.Client
func createHttpClient() *http.Client {

	transport := &http.Transport{
		Proxy:             http.ProxyFromEnvironment,
		DisableKeepAlives: true, // 关键修正：禁用连接复用，确保每次测试的独立性
		MaxIdleConns:      100, IdleConnTimeout: 90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true, // 开启HTTP/2尝试
	}

	return &http.Client{
		Transport: transport,
		// 禁止自动重定向，以便我们能分析单个请求的真实性能
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// performDnsTest 执行纯DNS查询测试
func performDnsTest(u *url.URL) (TraceResult, error) {
	var result TraceResult
	start := time.Now()
	ips, err := net.LookupHost(u.Hostname())
	if err != nil {
		return result, err
	}
	result.DNSLookup = time.Since(start)
	result.Total = result.DNSLookup
	result.ResolvedIPs = ips
	return result, nil
}

// performTcpTest 执行纯TCP连接测试
func performTcpTest(u *url.URL) (TraceResult, error) {
	var result TraceResult
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	// 为准确测量TCP连接时间，我们先在计时外解析DNS
	ips, err := net.LookupHost(u.Hostname())
	if err != nil {
		return result, fmt.Errorf("TCP测试前的DNS查询失败: %w", err)
	}
	result.ResolvedIPs = ips

	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ips[0], port), 5*time.Second)
	if err != nil {
		return result, err
	}
	conn.Close()
	result.TCPConnection = time.Since(start)
	result.Total = result.TCPConnection
	return result, nil
}

// performFullTrace 使用httptrace执行完整的HTTP生命周期跟踪
func performFullTrace(u *url.URL, client *http.Client) (TraceResult, error) {
	var result TraceResult
	// 声明用于记录各个阶段开始时间的变量
	var dnsStart, dnsDone, connStart, connDone, tlsStart, tlsDone, gotFirstByte, reqStart time.Time

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("User-Agent", "gemini-gonet-tester/1.0")

	// --- httptrace钩子设置 ---
	// 通过这些钩子函数，我们可以在HTTP请求的不同阶段捕获时间点

	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone: func(info httptrace.DNSDoneInfo) {
			dnsDone = time.Now()
			for _, addr := range info.Addrs {
				result.ResolvedIPs = append(result.ResolvedIPs, addr.String())
			}
		},
		ConnectStart:         func(network, addr string) { connStart = time.Now() },
		ConnectDone:          func(network, addr string, err error) { connDone = time.Now() },
		TLSHandshakeStart:    func() { tlsStart = time.Now() },
		TLSHandshakeDone:     func(state tls.ConnectionState, err error) { tlsDone = time.Now() },
		GotFirstResponseByte: func() { gotFirstByte = time.Now() },
	}

	req = req.WithContext(httptrace.WithClientTrace(context.Background(), trace))

	reqStart = time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	// 读取并丢弃响应体，以计算内容传输时间
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		// 对于计时而言，这不是一个致命错误
	}
	respDone := time.Now()

	// --- 计算各个阶段的耗时 ---
	result.DNSLookup = dnsDone.Sub(dnsStart)
	result.TCPConnection = connDone.Sub(connStart)
	// 如果是HTTPS请求，计算TLS握手时间
	if !tlsStart.IsZero() {
		result.TLSHandshake = tlsDone.Sub(tlsStart)
		result.ServerProcessing = gotFirstByte.Sub(tlsDone)
	} else {
		result.ServerProcessing = gotFirstByte.Sub(connDone)
	}
	result.ContentTransfer = respDone.Sub(gotFirstByte)
	result.Total = respDone.Sub(reqStart)

	return result, nil
}

// displaySingleRun 以美观的格式显示单次运行的结果
func displaySingleRun(r TraceResult, testType string) {
	color := timingColor(r.Total, 200*time.Millisecond, 1*time.Second)
	fmt.Printf("%s✓ %s分析 (Analysis)%s\n", ColorGreen, strings.ToUpper(testType), ColorReset)

	totalNanos := float64(r.Total.Nanoseconds())
	if totalNanos == 0 {
		totalNanos = 1 // 防止除以零
	}

	// 内部函数，用于打印带可视化条形图的单行结果
	printBar := func(name string, d time.Duration) {
		if d > 0 {
			// 计算该阶段耗时占总耗时的比例，并据此生成条形图
			ratio := float64(d.Nanoseconds()) / totalNanos
			barWidth := int(math.Ceil(ratio * 20)) // 20是条形图的最大宽度
			bar := strings.Repeat("█", barWidth)
			c := timingColor(d, 100*time.Millisecond, 500*time.Millisecond)
			fmt.Printf("├─ %-15s [%-20s] %s%8.2f ms%s\n", name, bar, c, float64(d.Milliseconds()), ColorReset)
		}
	}

	switch testType {
	case "all":
		printBar("DNS 查询", r.DNSLookup)
		printBar("TCP 连接", r.TCPConnection)
		if r.TLSHandshake > 0 {
			printBar("TLS 握手", r.TLSHandshake)
		}
		printBar("服务器处理 (TTFB)", r.ServerProcessing)
		printBar("内容传输", r.ContentTransfer)
	case "dns":
		fmt.Printf("├─ %-15s: %s%.2f ms%s\n", "解析耗时", color, float64(r.DNSLookup.Milliseconds()), ColorReset)
	case "tcp":
		fmt.Printf("├─ %-15s: %s%.2f ms%s\n", "连接耗时", color, float64(r.TCPConnection.Milliseconds()), ColorReset)
	}

	fmt.Printf("\n%s✓ %s总体统计 (Overall Stats)%s\n", ColorGreen, strings.ToUpper(testType), ColorReset)
	fmt.Printf("├─ %-15s: %s%.2f ms%s\n", "总耗时", color, float64(r.Total.Milliseconds()), ColorReset)
	if len(r.ResolvedIPs) > 0 {
		fmt.Printf("└─ %-15s: %s\n", "解析IP", strings.Join(r.ResolvedIPs, ", "))
	}
}

// displaySummary 显示多次运行的统计摘要
func displaySummary(results []TraceResult, testType string) {
	fmt.Printf("%s✓ 测试结果汇总 (Summary of %d runs)%s\n\n", ColorGreen, len(results), ColorReset)

	// 内部函数，用于计算并打印单个指标的统计数据
	printStats := func(name string, times []time.Duration) {
		if len(times) < 1 {
			return
		}
		stats := calculateStats(times)
		c := timingColor(stats.Avg, 100*time.Millisecond, 500*time.Millisecond)

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

	// 提取并打印每个指标的统计
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

// calculateStats 计算一组时间数据的最小、最大和平均值
func calculateStats(runs []time.Duration) Statistics {
	if len(runs) == 0 {
		return Statistics{}
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i] < runs[j] })
	var total time.Duration
	for _, r := range runs {
		total += r
	}
	return Statistics{
		Min:     runs[0],
		Max:     runs[len(runs)-1],
		Avg:     total / time.Duration(len(runs)),
		AllRuns: runs,
	}
}

// timingColor 根据耗时返回不同的颜色，用于视觉提示
func timingColor(d, good, bad time.Duration) string {
	if d < good {
		return ColorGreen
	}
	if d < bad {
		return ColorYellow
	}
	return ColorRed
}

// printError 以标准错误格式打印红色的错误信息
func printError(msg string) {
	fmt.Fprintf(os.Stderr, "%s✗ 错误: %s%s\n", ColorRed, msg, ColorReset)
}
