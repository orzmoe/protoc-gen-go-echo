package main

import "testing"

var (
	benchmarkStringValue  string
	benchmarkStringsValue []string
	benchmarkErrorValue   error
)

func BenchmarkNormalizeEchoPath(b *testing.B) {
	// 测试常见路径模式
	paths := []string{
		"/v1/items/{id}",
		"/v1/items/{id=*}",
		"/v1/items/list",
		"/v1/{name=projects/*/items/*}", // 这个会返回 error，也测
	}
	for _, path := range paths {
		b.Run(path, func(b *testing.B) {
			for b.Loop() {
				benchmarkStringValue, benchmarkErrorValue = normalizeEchoPath(path)
			}
		})
	}
}

func BenchmarkToSnakeCase(b *testing.B) {
	inputs := []string{
		"GetBlogArticles",
		"HTTPRequest",
		"CreateUserProfile",
	}
	for _, input := range inputs {
		b.Run(input, func(b *testing.B) {
			for b.Loop() {
				benchmarkStringValue = toSnakeCase(input)
			}
		})
	}
}

func BenchmarkSanitizeImportAlias(b *testing.B) {
	inputs := []string{
		"mypkg",
		"pkg-name/v1",
		"9pkg",
	}
	for _, input := range inputs {
		b.Run(input, func(b *testing.B) {
			for b.Loop() {
				benchmarkStringValue = sanitizeImportAlias(input)
			}
		})
	}
}

func BenchmarkSplitCommaSeparated(b *testing.B) {
	inputs := []string{
		"perm.a, perm.b, perm.c",
		"single",
		"a, , b, , c",
	}
	for _, input := range inputs {
		b.Run(input, func(b *testing.B) {
			for b.Loop() {
				benchmarkStringsValue = splitCommaSeparated(input)
			}
		})
	}
}

func BenchmarkParseCommentsContent(b *testing.B) {
	content := "// @summary 获取用户列表\n// @description 根据筛选条件返回分页用户列表\n// @description 支持按角色和状态筛选"
	b.ResetTimer()
	for b.Loop() {
		benchmarkStringValue, _ = parseCommentsContent(content)
	}
}
