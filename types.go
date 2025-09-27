package main

import "time"

// 用于终端彩色输出的ANSI颜色代码
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
	ColorBold   = "\033[1m"
)

// TraceResult 存放单次网络跟踪的所有计时结果
type TraceResult struct {
	DNSLookup        time.Duration // DNS查询耗时
	TCPConnection    time.Duration // TCP连接耗时
	TLSHandshake     time.Duration // TLS握手耗时
	ServerProcessing time.Duration // 服务器处理耗时 (从请求发送完毕到接收到第一个字节)
	ContentTransfer  time.Duration // 内容传输耗时 (从接收到第一个字节到响应全部接收完毕)
	Total            time.Duration // 总耗时 (从请求开始到响应全部接收完毕)
	ResolvedIPs      []string      // DNS查询解析出的IP地址列表
}

// Statistics 存放多次运行结果的统计数据
type Statistics struct {
	Min     time.Duration   // 所有运行中的最快耗时
	Max     time.Duration   // 所有运行中的最慢耗时
	Avg     time.Duration   // 所有运行的平均耗时
	AllRuns []time.Duration // 每次运行的具体耗时记录
}
