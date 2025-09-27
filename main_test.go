package main

import (
	"testing"
)

// TestNormalizeURL 是对 normalizeURL 函数的单元测试
func TestNormalizeURL(t *testing.T) {
	// 定义一个测试用例的结构体，增加了 useHTTP 字段
	testCases := []struct {
		name        string // 测试用例名称
		input       string // 输入的原始URL
		useHTTP     bool   // 模拟 -http 标志
		expectedURL string // 期望得到的URL字符串
		expectError bool   // 是否期望出现错误
	}{
		{
			name:        "Valid HTTPS URL",
			input:       "https://google.com",
			useHTTP:     false,
			expectedURL: "https://google.com",
			expectError: false,
		},
		{
			name:        "URL without scheme (default to https)",
			input:       "google.com",
			useHTTP:     false,
			expectedURL: "https://google.com",
			expectError: false,
		},
		{
			name:        "URL without scheme and -http flag",
			input:       "example.com",
			useHTTP:     true, // 模拟 -http 标志
			expectedURL: "http://example.com",
			expectError: false,
		},
		{
			name:        "Valid HTTP URL",
			input:       "http://example.com",
			useHTTP:     false,
			expectedURL: "http://example.com",
			expectError: false,
		},
		{
			name:        "Invalid URL format",
			input:       "://invalid",
			useHTTP:     false,
			expectError: true,
		},
		{
			name:        "Empty string input",
			input:       "",
			useHTTP:     false,
			expectError: true,
		},
	}

	// 遍历所有测试用例
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 调用更新后的 normalizeURL 函数
			parsedURL, err := normalizeURL(tc.input, tc.useHTTP)

			// 检查错误是否符合预期
			hasError := err != nil
			if hasError != tc.expectError {
				t.Errorf("Expected error: %v, but got error: %v", tc.expectError, err)
			}

			// 如果不期望出错，则进一步检查URL是否被正确规范化
			if !tc.expectError && parsedURL.String() != tc.expectedURL {
				t.Errorf("Expected URL '%s', but got '%s'", tc.expectedURL, parsedURL.String())
			}
		})
	}
}
