package main

import (
	"slices"
	"strings"
	"testing"
)

func TestUnit_normalizeEchoPath(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        string
		errContains string
	}{
		{
			name:  "空字符串",
			input: "",
			want:  "",
		},
		{
			name:  "普通路径",
			input: "/v1/items/list",
			want:  "/v1/items/list",
		},
		{
			name:  "花括号参数",
			input: "/v1/items/{id}",
			want:  "/v1/items/:id",
		},
		{
			name:  "星号通配参数",
			input: "/v1/items/{id=*}",
			want:  "/v1/items/:id",
		},
		{
			name:  "已是 echo 参数",
			input: "/v1/items/:id",
			want:  "/v1/items/:id",
		},
		{
			name:        "空参数名",
			input:       "/v1/items/{}",
			errContains: "非法的路径参数片段",
		},
		{
			name:        "嵌套花括号错误",
			input:       "/v1/items/{id}}",
			errContains: "非法的路径参数片段",
		},
		{
			name:        "复杂路径模板",
			input:       "/v1/{name=projects/*/items/*}",
			errContains: "暂不支持复杂路径模板",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeEchoPath(tc.input)
			if tc.errContains != "" {
				if err == nil {
					t.Fatalf("期望返回错误，实际为 nil，输入: %q", tc.input)
				}
				if !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("错误信息不匹配，期望包含 %q，实际: %v", tc.errContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("不期望返回错误，实际: %v", err)
			}
			if got != tc.want {
				t.Fatalf("归一化结果不匹配，期望 %q，实际 %q", tc.want, got)
			}
		})
	}
}

func TestUnit_splitPathSegments(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        []string
		errContains string
	}{
		{
			name:  "空字符串",
			input: "",
			want:  []string{""},
		},
		{
			name:  "根路径",
			input: "/v1/items",
			want:  []string{"", "v1", "items"},
		},
		{
			name:  "含花括号参数",
			input: "/v1/items/{id}",
			want:  []string{"", "v1", "items", "{id}"},
		},
		{
			name:  "含通配参数",
			input: "/v1/{name=projects/*/items/*}",
			want:  []string{"", "v1", "{name=projects/*/items/*}"},
		},
		{
			name:        "花括号不匹配",
			input:       "/v1/items/{id",
			errContains: "非法的路径参数片段",
		},
		{
			name:        "多余右花括号",
			input:       "/v1/items/id}",
			errContains: "非法的路径参数片段",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := splitPathSegments(tc.input)
			if tc.errContains != "" {
				if err == nil {
					t.Fatalf("期望返回错误，实际为 nil")
				}
				if !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("错误信息不匹配，期望包含 %q，实际: %v", tc.errContains, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("不期望返回错误，实际: %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("切分结果不匹配，期望 %#v，实际 %#v", tc.want, got)
			}
		})
	}
}
