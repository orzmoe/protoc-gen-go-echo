package main

import (
	"google.golang.org/protobuf/compiler/protogen"
	"testing"
)

func TestUnit_sanitizeImportAlias(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "空字符串",
			input: "",
			want:  "pkg",
		},
		{
			name:  "正常标识符",
			input: "mypkg",
			want:  "mypkg",
		},
		{
			name:  "数字开头",
			input: "9pkg",
			want:  "_9pkg",
		},
		{
			name:  "含特殊字符",
			input: "pkg-name/v1",
			want:  "pkg_name_v1",
		},
		{
			name:  "仅特殊字符",
			input: "-.-",
			want:  "___",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeImportAlias(tc.input)
			if got != tc.want {
				t.Fatalf("import alias 清洗结果不匹配，期望 %q，实际 %q", tc.want, got)
			}
		})
	}
}

func TestUnit_importManagerAliasCollision(t *testing.T) {
	pkgNames := map[protogen.GoImportPath]protogen.GoPackageName{
		"example.com/pkg/auth":    "auth",
		"example.com/pkg/auth/v2": "auth",
		"example.com/pkg/echo":    "echo",
	}

	mgr := newImportManager("example.com/current", pkgNames, "echo", "context")

	// 第一个 auth 包正常使用 "auth"
	alias1 := mgr.aliasForImportPath("example.com/pkg/auth")
	if alias1 != "auth" {
		t.Fatalf("第一个 auth 包别名应为 auth，实际: %q", alias1)
	}

	// 第二个 auth 包应自动编号为 "auth2"
	alias2 := mgr.aliasForImportPath("example.com/pkg/auth/v2")
	if alias2 != "auth2" {
		t.Fatalf("第二个 auth 包别名应为 auth2，实际: %q", alias2)
	}

	// echo 已被保留，应编号为 "echo2"
	alias3 := mgr.aliasForImportPath("example.com/pkg/echo")
	if alias3 != "echo2" {
		t.Fatalf("echo 已保留，第三方 echo 包应为 echo2，实际: %q", alias3)
	}

	// 重复请求应返回缓存值
	alias1Again := mgr.aliasForImportPath("example.com/pkg/auth")
	if alias1Again != alias1 {
		t.Fatalf("重复请求应返回缓存值，期望 %q，实际 %q", alias1, alias1Again)
	}

	// 当前文件自身路径应返回空
	selfAlias := mgr.aliasForImportPath("example.com/current")
	if selfAlias != "" {
		t.Fatalf("当前文件自身路径应返回空，实际: %q", selfAlias)
	}

	// Imports 应返回排序结果
	imports := mgr.Imports()
	if len(imports) != 3 {
		t.Fatalf("应有 3 个 import，实际: %d", len(imports))
	}
	// 按 alias 排序: auth, auth2, echo2
	if imports[0].Alias != "auth" || imports[1].Alias != "auth2" || imports[2].Alias != "echo2" {
		t.Fatalf("Imports 应按 alias 排序，实际: %v", imports)
	}
}

func TestUnit_importManagerReservedServiceNames(t *testing.T) {
	// 模拟 MF-2 修复：服务名被保留后，同名外部包应自动编号
	pkgNames := map[protogen.GoImportPath]protogen.GoPackageName{
		"example.com/external/auth": "AuthService",
	}

	mgr := newImportManager("example.com/current", pkgNames, "AuthService", "AuthServiceHTTPServer")

	alias := mgr.aliasForImportPath("example.com/external/auth")
	if alias == "AuthService" {
		t.Fatalf("已保留的服务名不应作为 import alias，实际: %q", alias)
	}
	if alias != "AuthService2" {
		t.Fatalf("冲突时应自动编号为 AuthService2，实际: %q", alias)
	}
}
