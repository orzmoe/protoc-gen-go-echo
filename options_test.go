package main

import (
	"slices"
	"strings"
	"testing"
)

func TestUnit_splitCommaSeparated(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantNil bool
	}{
		{
			name:    "空字符串",
			input:   "",
			wantNil: true,
		},
		{
			name:  "单值",
			input: "a",
			want:  []string{"a"},
		},
		{
			name:  "多值",
			input: "a,b,c",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "包含空格",
			input: "a, b",
			want:  []string{"a", "b"},
		},
		{
			name:  "尾逗号",
			input: "a,,",
			want:  []string{"a"},
		},
		{
			name:    "全空格",
			input:   "  ",
			wantNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitCommaSeparated(tc.input)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("期望返回 nil，实际: %#v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("不期望返回 nil，输入: %q", tc.input)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("切分结果不匹配，期望 %#v，实际 %#v", tc.want, got)
			}
		})
	}
}

func TestUnit_PermissionDisplay(t *testing.T) {
	tests := []struct {
		name string
		m    method
		want string
	}{
		{
			name: "全部为空",
			m:    method{},
			want: "",
		},
		{
			name: "仅 Permission",
			m: method{
				Permission: "perm.read",
			},
			want: "perm.read",
		},
		{
			name: "仅 Permissions",
			m: method{
				Permissions: "perm.a,perm.b",
			},
			want: "perm.a,perm.b",
		},
		{
			name: "仅 AnyPermission",
			m: method{
				AnyPermission: "perm.any",
			},
			want: "perm.any",
		},
		{
			name: "多个同时设置时取第一个",
			m: method{
				Permission:    "perm.first",
				Permissions:   "perm.second",
				AnyPermission: "perm.third",
			},
			want: "perm.first",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.m.PermissionDisplay()
			if got != tc.want {
				t.Fatalf("展示权限不匹配，期望 %q，实际 %q", tc.want, got)
			}
		})
	}
}

func TestUnit_normalizeMethodOptions(t *testing.T) {
	tests := []struct {
		name           string
		opts           methodOptions
		wantPermission string
		wantPerms      string
		wantAnyPerm    string
		errContains    string
	}{
		{
			name: "全部为空不报错",
			opts: methodOptions{},
		},
		{
			name:           "permission 正常值 trim",
			opts:           methodOptions{permission: "  perm.read  "},
			wantPermission: "perm.read",
		},
		{
			name:        "permission 纯空白报错",
			opts:        methodOptions{permission: "   "},
			errContains: "permission 值不能为纯空白",
		},
		{
			name:      "permissions 正常值 trim 各项",
			opts:      methodOptions{permissions: " a , b , c "},
			wantPerms: "a,b,c",
		},
		{
			name:        "permissions 全逗号空白报错",
			opts:        methodOptions{permissions: " , , "},
			errContains: "permissions 值不能为纯空白或空列表",
		},
		{
			name:      "permissions 过滤空项后保留有效值",
			opts:      methodOptions{permissions: "a, , b"},
			wantPerms: "a,b",
		},
		{
			name:        "anyPermission 纯空白报错",
			opts:        methodOptions{anyPermission: "  "},
			errContains: "any_permission 值不能为纯空白或空列表",
		},
		{
			name:        "anyPermission 全逗号空白报错",
			opts:        methodOptions{anyPermission: " , "},
			errContains: "any_permission 值不能为纯空白或空列表",
		},
		{
			name:        "anyPermission 正常值 trim",
			opts:        methodOptions{anyPermission: " x , y "},
			wantAnyPerm: "x,y",
		},
		{
			name:      "permissions 去重",
			opts:      methodOptions{permissions: "a, b, a, c, b"},
			wantPerms: "a,b,c",
		},
		{
			name:        "anyPermission 去重",
			opts:        methodOptions{anyPermission: "x, x, y"},
			wantAnyPerm: "x,y",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := tc.opts
			err := normalizeMethodOptions(&opts)
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
			if tc.wantPermission != "" && opts.permission != tc.wantPermission {
				t.Fatalf("permission 规范化结果不匹配，期望 %q，实际 %q", tc.wantPermission, opts.permission)
			}
			if tc.wantPerms != "" && opts.permissions != tc.wantPerms {
				t.Fatalf("permissions 规范化结果不匹配，期望 %q，实际 %q", tc.wantPerms, opts.permissions)
			}
			if tc.wantAnyPerm != "" && opts.anyPermission != tc.wantAnyPerm {
				t.Fatalf("anyPermission 规范化结果不匹配，期望 %q，实际 %q", tc.wantAnyPerm, opts.anyPermission)
			}
		})
	}
}

func TestUnit_validateMethodOptions(t *testing.T) {
	tests := []struct {
		name        string
		opts        methodOptions
		errContains string
	}{
		{
			name: "无权限配置",
			opts: methodOptions{},
		},
		{
			name: "仅 public",
			opts: methodOptions{public: true},
		},
		{
			name: "仅 permission",
			opts: methodOptions{permission: "perm.read"},
		},
		{
			name: "public 与 authOnly 冲突",
			opts: methodOptions{
				public:   true,
				authOnly: true,
			},
			errContains: "public, auth_only",
		},
		{
			name: "三项冲突",
			opts: methodOptions{
				public:      true,
				permission:  "perm.single",
				permissions: "perm.a,perm.b",
			},
			errContains: "public, permission, permissions",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMethodOptions(tc.opts)
			if tc.errContains != "" {
				if err == nil {
					t.Fatalf("期望返回冲突错误，实际为 nil")
				}
				if !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("错误信息不匹配，期望包含 %q，实际 %v", tc.errContains, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("不期望返回错误，实际: %v", err)
			}
		})
	}
}

func TestUnit_deduplicateStrings(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "nil 输入",
			input: nil,
			want:  nil,
		},
		{
			name:  "空切片",
			input: []string{},
			want:  nil,
		},
		{
			name:  "无重复",
			input: []string{"a", "b", "c"},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "有重复保序",
			input: []string{"a", "b", "a", "c", "b"},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "全部相同",
			input: []string{"x", "x", "x"},
			want:  []string{"x"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := deduplicateStrings(tc.input)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("期望返回 nil，实际: %#v", got)
				}
				return
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("去重结果不匹配，期望 %#v，实际 %#v", tc.want, got)
			}
		})
	}
}
