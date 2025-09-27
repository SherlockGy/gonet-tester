package main

import (
	"testing"
)

// TestNormalizeURL 是对 normalizeURL 函数的单元测试
func TestNormalizeURL(t *testing.T) {
	// 定义一个测试用例的结构体
	testCases := []struct {
		name        string // 测试用例名称
		input       string // 输入的原始URL
		expectedURL string // 期望得到的URL字符串
		expectError bool   // 是否期望出现错误
	}{
		{
			name:        "Valid HTTPS URL",
			input:       "https://google.com",
			expectedURL: "https://google.com",
			expectError: false,
		},
		{
			name:        "URL without scheme",
			input:       "google.com",
			expectedURL: "https://google.com",
			expectError: false,
		},
		{
			name:        "Valid HTTP URL",
			input:       "http://example.com",
			expectedURL: "http://example.com",
			expectError: false,
		},
		{
			name:        "Invalid URL format",
			input:       "://invalid",
			expectError: true,
		},
		{
			name:        "Empty string input",
			input:       "",
			expectError: true,
		},
	}

	// 遍历所有测试用例
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsedURL, err := normalizeURL(tc.input)

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
