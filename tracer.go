package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"time"
)

// performDnsTest 执行纯DNS查询测试。
// u: 已解析的URL对象，函数将使用其主机名进行DNS查询。
func performDnsTest(u *url.URL) (TraceResult, error) {
	var result TraceResult
	start := time.Now()
	ips, err := net.LookupHost(u.Hostname())
	if err != nil {
		return result, err
	}
	result.DNSLookup = time.Since(start)
	result.Total = result.DNSLookup // 对于纯DNS测试，总耗时即DNS查询耗时
	result.ResolvedIPs = ips
	return result, nil
}

// performTcpTest 执行纯TCP连接测试。
// u: 已解析的URL对象，函数将根据其协议和主机名进行TCP连接。
func performTcpTest(u *url.URL) (TraceResult, error) {
	var result TraceResult
	port := u.Port()
	// 如果URL中没有指定端口，则根据协议使用默认端口
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	// 为准确测量TCP连接时间，我们先在计时之外解析出IP地址
	ips, err := net.LookupHost(u.Hostname())
	if err != nil {
		return result, fmt.Errorf("TCP测试前的DNS查询失败: %w", err)
	}
	result.ResolvedIPs = ips

	// 开始计时并建立TCP连接
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ips[0], port), 5*time.Second)
	if err != nil {
		return result, err
	}
	_ = conn.Close() // 连接成功后立即关闭
	result.TCPConnection = time.Since(start)
	result.Total = result.TCPConnection // 对于纯TCP测试，总耗时即TCP连接耗时
	return result, nil
}

// performFullTrace 使用httptrace执行完整的HTTP生命周期跟踪。
// u: 已解析的URL对象。
// client: 一个配置好的、可复用的http.Client实例。
func performFullTrace(u *url.URL, client *http.Client) (TraceResult, error) {
	var result TraceResult
	// 声明用于记录各个阶段开始和结束时间的变量
	var dnsStart, dnsDone, connStart, connDone, tlsStart, tlsDone, gotFirstByte, reqStart time.Time

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("User-Agent", "gemini-gonet-tester/1.0")

	// --- httptrace钩子设置 ---
	// 通过实现这些钩子函数，我们可以在HTTP请求的不同阶段捕获精确的时间点
	trace := &httptrace.ClientTrace{
		// DNS查询开始
		DNSStart: func(info httptrace.DNSStartInfo) { dnsStart = time.Now() },
		// DNS查询结束
		DNSDone: func(info httptrace.DNSDoneInfo) {
			dnsDone = time.Now()
			for _, addr := range info.Addrs {
				result.ResolvedIPs = append(result.ResolvedIPs, addr.String())
			}
		},
		// 建立TCP连接开始（在DNS查询之后）
		ConnectStart: func(network, addr string) { connStart = time.Now() },
		// 建立TCP连接结束
		ConnectDone: func(network, addr string, err error) { connDone = time.Now() },
		// TLS握手开始（仅HTTPS）
		TLSHandshakeStart: func() { tlsStart = time.Now() },
		// TLS握手结束（仅HTTPS）
		TLSHandshakeDone: func(state tls.ConnectionState, err error) { tlsDone = time.Now() },
		// 接收到响应的第一个字节
		GotFirstResponseByte: func() { gotFirstByte = time.Now() },
	}

	// 将跟踪器附加到请求的context中
	req = req.WithContext(httptrace.WithClientTrace(context.Background(), trace))

	// 记录请求的开始时间，并发送请求
	reqStart = time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	// 读取并丢弃响应体，这是为了完整地计算内容传输时间
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		// 在计时场景下，读取响应体的错误通常不认为是致命的
	}
	// 记录响应全部接收完毕的时间
	respDone := time.Now()

	// --- 根据记录的时间点，计算各个阶段的耗时 ---
	result.DNSLookup = dnsDone.Sub(dnsStart)
	result.TCPConnection = connDone.Sub(connStart)

	// 如果是HTTPS请求(tlsStart被记录过)，则计算TLS握手时间和服务器处理时间
	if !tlsStart.IsZero() {
		result.TLSHandshake = tlsDone.Sub(tlsStart)
		result.ServerProcessing = gotFirstByte.Sub(tlsDone)
	} else { // 如果是HTTP请求
		result.ServerProcessing = gotFirstByte.Sub(connDone)
	}
	result.ContentTransfer = respDone.Sub(gotFirstByte)
	result.Total = respDone.Sub(reqStart)

	return result, nil
}
