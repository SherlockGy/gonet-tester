package main

import (
	"flag"
	"fmt"
	"os"

	"gonet-tester/internal"
)

// main 是程序的入口函数
func main() {
	// --- 1. 参数解析 ---
	// 定义命令行接受的所有标志 (flag)
	testType := flag.String("test", "all", "要运行的测试类型 (all, dns, tcp)")
	runCount := flag.Int("n", 1, "测试运行的次数")
	httpFlag := flag.Bool("http", false, "对于不带协议的域名，优先使用http而非https")

	// 自定义用法说明
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s用法: gonet-tester [参数] <url>%s\n", internal.ColorBold, internal.ColorReset)
		fmt.Fprintf(os.Stderr, "示例: gonet-tester -http -n 5 google.com\n\n")
		fmt.Fprintf(os.Stderr, "参数:\n")
		flag.PrintDefaults()
	}
	// 解析命令行传入的参数
	flag.Parse()

	// 检查是否提供了URL参数
	if len(flag.Args()) != 1 {
		flag.Usage()
		os.Exit(1)
	}

	// --- 2. URL处理 ---
	// 调用辅助函数规范化URL
	parsedURL, err := internal.NormalizeURL(flag.Args()[0], *httpFlag)
	if err != nil {
		internal.PrintError(err.Error())
		os.Exit(1)
	}

	fmt.Printf("\n%s正在对 %s 进行 %d 次 %s 测试...%s\n\n", internal.ColorBold, parsedURL.String(), *runCount, *testType, internal.ColorReset)

	// --- 3. 执行测试循环 ---
	var results []internal.TraceResult
	// 创建一个可复用的HTTP客户端
	client := internal.CreateHttpClient()

	for i := 0; i < *runCount; i++ {
		var result internal.TraceResult
		var err error

		// 根据测试类型，调用不同的核心测试函数
		switch *testType {
		case "dns":
			result, err = internal.PerformDnsTest(parsedURL)
		case "tcp":
			result, err = internal.PerformTcpTest(parsedURL)
		case "all":
			result, err = internal.PerformFullTrace(parsedURL, client)
		default:
			internal.PrintError(fmt.Sprintf("无效的测试类型: %s. 合法值是: all, dns, tcp.", *testType))
			os.Exit(1)
		}

		// 处理单次运行的错误
		if err != nil {
			// 修正: 使用 %v 而不是 %%v
			internal.PrintError(fmt.Sprintf("第 %d 次测试失败: %v", i+1, err))
			// 如果是完全失败，则跳过此次结果
			if result.Total == 0 {
				continue
			}
		}
		results = append(results, result)
	}

	// 如果所有测试都失败了，则退出
	if len(results) == 0 {
		// 修正: 移除错误的希伯来字符
		internal.PrintError("所有测试均失败。\n")
		os.Exit(1)
	}

	// --- 4. 显示结果 ---
	// 根据运行次数，调用不同的显示函数
	if *runCount == 1 {
		internal.DisplaySingleRun(results[0], *testType)
	} else {
		internal.DisplaySummary(results, *testType)
	}
}
