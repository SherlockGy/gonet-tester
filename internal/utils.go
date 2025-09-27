package internal

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// NormalizeURL 规范化用户输入的URL，进行严格检查，并根据参数决定协议头。
// rawURL: 用户输入的原始URL字符串。
// useHTTP: 是否在没有协议头时优先使用http。
func NormalizeURL(rawURL string, useHTTP bool) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("无效的URL: 输入为空")
	}

	// 处理协议相对URL (例如, "//google.com")
	if strings.HasPrefix(rawURL, "//") {
		rawURL = "https:" + rawURL
	}

	// 如果仍然没有协议头，则根据 useHTTP 标志添加 http:// 或 https://
	if !strings.Contains(rawURL, "://") {
		if useHTTP {
			rawURL = "http://" + rawURL
		} else {
			rawURL = "https://" + rawURL
		}
	}

	// 解析最终生成的URL字符串
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("无效的URL: 解析失败 - %s (%w)", rawURL, err)
	}

	// 强制检查协议是否为 http 或 https
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("无效的URL: 仅支持http和https协议 (检测到: %s)", parsedURL.Scheme)
	}

	// 强制检查主机名是否存在
	if parsedURL.Host == "" {
		return nil, fmt.Errorf("无效的URL: 缺少主机名 - %s", rawURL)
	}

	return parsedURL, nil
}

// CreateHttpClient 创建并配置一个可复用的http.Client。
// 此处禁用了连接复用(Keep-Alive)，以确保每次测试都是一个全新的连接，从而保证测试结果的准确性。
func CreateHttpClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DisableKeepAlives:     true, // 关键：禁用连接复用，确保每次测试的独立性
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true, // 开启HTTP/2尝试
	}

	return &http.Client{
		Transport: transport,
		// 禁止客户端自动处理重定向，以便我们能分析单个请求的真实性能
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// CalculateStats 计算一组时间数据的最小、最大和平均值。
// runs: 包含多次运行耗时的切片。
func CalculateStats(runs []time.Duration) Statistics {
	if len(runs) == 0 {
		return Statistics{}
	}
	// 排序以方便找到最小值和最大值
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
