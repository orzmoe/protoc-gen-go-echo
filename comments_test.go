package main

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/compiler/protogen"
)

func TestUnit_parseCommentsContent(t *testing.T) {
	tests := []struct {
		name            string
		comments        string
		wantSummary     string
		wantDescription string
	}{
		{
			name:            "空注释",
			comments:        "",
			wantSummary:     "",
			wantDescription: "",
		},
		{
			name:            "仅 summary",
			comments:        "// @summary 获取用户",
			wantSummary:     "获取用户",
			wantDescription: "",
		},
		{
			name: "summary 加单行 description",
			comments: strings.Join([]string{
				"// @summary 创建用户",
				"// @description 根据请求参数创建用户",
			}, "\n"),
			wantSummary:     "创建用户",
			wantDescription: "根据请求参数创建用户",
		},
		{
			name: "多行 description 合并",
			comments: strings.Join([]string{
				"// @summary 批量导出",
				"// @description 第一步过滤数据",
				"// @description 第二步导出文件",
			}, "\n"),
			wantSummary:     "批量导出",
			wantDescription: "第一步过滤数据 第二步导出文件",
		},
		{
			name: "无标签内容",
			comments: strings.Join([]string{
				"// 这是一段普通注释",
				"// 没有任何 summary/description 标签",
			}, "\n"),
			wantSummary:     "",
			wantDescription: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotSummary, gotDescription := parseCommentsContent(tc.comments)
			if gotSummary != tc.wantSummary {
				t.Fatalf("summary 解析不匹配，期望 %q，实际 %q", tc.wantSummary, gotSummary)
			}
			if gotDescription != tc.wantDescription {
				t.Fatalf("description 解析不匹配，期望 %q，实际 %q", tc.wantDescription, gotDescription)
			}
		})
	}
}

func TestUnit_resolveVersion(t *testing.T) {
	originalVersion := version
	t.Cleanup(func() {
		version = originalVersion
	})

	tests := []struct {
		name     string
		injected string
		want     string
		validate func(t *testing.T, got string)
	}{
		{
			name:     "ldflags 注入语义版本",
			injected: "v1.2.3",
			want:     "v1.2.3",
		},
		{
			name:     "ldflags 注入任意版本标识",
			injected: "feature-build",
			want:     "feature-build",
		},
		{
			name:     "未注入时走 BuildInfo fallback",
			injected: "",
			validate: func(t *testing.T, got string) {
				t.Helper()
				if got == "" {
					t.Fatalf("未注入版本时不应返回空字符串")
				}
				if got != "(devel)" && !strings.HasPrefix(got, "v") {
					t.Fatalf("fallback 结果格式异常，实际 %q", got)
				}
				if gotAgain := resolveVersion(); gotAgain != got {
					t.Fatalf("多次调用结果应一致，第一次 %q，第二次 %q", got, gotAgain)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			version = tc.injected
			got := resolveVersion()
			if tc.want != "" && got != tc.want {
				t.Fatalf("版本解析结果不匹配，期望 %q，实际 %q", tc.want, got)
			}
			if tc.validate != nil {
				tc.validate(t, got)
			}
		})
	}
}

func assertPackageNameRegistered(t *testing.T, packageNames map[protogen.GoImportPath]protogen.GoPackageName, importPath protogen.GoImportPath, want protogen.GoPackageName, wantExists bool) {
	t.Helper()
	got, exists := packageNames[importPath]
	if exists != wantExists {
		t.Fatalf("包注册存在性不匹配，路径 %q，期望存在=%v，实际存在=%v", importPath, wantExists, exists)
	}
	if wantExists && got != want {
		t.Fatalf("包名注册结果不匹配，路径 %q，期望 %q，实际 %q", importPath, want, got)
	}
}

func TestUnit_registerGoPackageName(t *testing.T) {
	tests := []struct {
		name        string
		file        *protogen.File
		checkPath   protogen.GoImportPath
		wantName    protogen.GoPackageName
		wantExists  bool
		seedMapping map[protogen.GoImportPath]protogen.GoPackageName
	}{
		{
			name:        "nil file",
			file:        nil,
			checkPath:   "example.com/existing",
			wantName:    "existingpkg",
			wantExists:  true,
			seedMapping: map[protogen.GoImportPath]protogen.GoPackageName{"example.com/existing": "existingpkg"},
		},
		{
			name: "空包名",
			file: &protogen.File{
				GoImportPath:  "example.com/test/empty",
				GoPackageName: "",
			},
			checkPath:  "example.com/test/empty",
			wantExists: false,
		},
		{
			name: "正常场景",
			file: &protogen.File{
				GoImportPath:  "example.com/test/v1",
				GoPackageName: "testv1",
			},
			checkPath:  "example.com/test/v1",
			wantName:   "testv1",
			wantExists: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			packageNames := make(map[protogen.GoImportPath]protogen.GoPackageName)
			for path, name := range tc.seedMapping {
				packageNames[path] = name
			}

			registerGoPackageName(packageNames, tc.file)
			assertPackageNameRegistered(t, packageNames, tc.checkPath, tc.wantName, tc.wantExists)
		})
	}
}

func TestUnit_registerWellKnownPackage(t *testing.T) {
	tests := []struct {
		name       string
		importPath protogen.GoImportPath
		pkgName    protogen.GoPackageName
		wantExists bool
	}{
		{
			name:       "空值",
			importPath: "",
			pkgName:    "emptypb",
			wantExists: false,
		},
		{
			name:       "正常值",
			importPath: "google.golang.org/protobuf/types/known/emptypb",
			pkgName:    "emptypb",
			wantExists: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			packageNames := make(map[protogen.GoImportPath]protogen.GoPackageName)
			registerWellKnownPackage(packageNames, tc.importPath, tc.pkgName)

			assertPackageNameRegistered(t, packageNames, tc.importPath, tc.pkgName, tc.wantExists)
		})
	}
}

func TestUnit_toSnakeCase(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "CamelCase",
			input: "CreateUserProfile",
			want:  "create_user_profile",
		},
		{
			name:  "连续大写",
			input: "HTTPRequest",
			want:  "http_request",
		},
		{
			name:  "含数字",
			input: "API2JSON",
			want:  "api2_json",
		},
		{
			name:  "含连字符",
			input: "user-id",
			want:  "user_id",
		},
		{
			name:  "已含下划线",
			input: "already_snake",
			want:  "already_snake",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := toSnakeCase(tc.input)
			if got != tc.want {
				t.Fatalf("snake_case 转换不匹配，期望 %q，实际 %q", tc.want, got)
			}
		})
	}
}
