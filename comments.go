package main

import (
	"regexp"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

var (
	matchFirstCap = regexp.MustCompile("([A-Z])([A-Z][a-z])")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)

// toSnakeCase 将 CamelCase/PascalCase 转换为 snake_case。
// 例如: GetBlogArticles → get_blog_articles, HTTPRequest → http_request
func toSnakeCase(input string) string {
	output := matchFirstCap.ReplaceAllString(input, "${1}_${2}")
	output = matchAllCap.ReplaceAllString(output, "${1}_${2}")
	output = strings.ReplaceAll(output, "-", "_")
	return strings.ToLower(output)
}

var (
	summaryRegex     = regexp.MustCompile(`@summary\s+(.+)`)
	descriptionRegex = regexp.MustCompile(`@description\s+(.+)`)
)

// collectMethodComments 统一收集方法的所有注释源（LeadingDetached + Leading + Trailing），
// 返回合并后的字符串。所有依赖注释内容的逻辑都应通过此函数获取，保证语义一致。
func collectMethodComments(m *protogen.Method) string {
	var allComments []string
	for _, detached := range m.Comments.LeadingDetached {
		allComments = append(allComments, detached.String())
	}
	if leading := m.Comments.Leading.String(); leading != "" {
		allComments = append(allComments, leading)
	}
	if trailing := m.Comments.Trailing.String(); trailing != "" {
		allComments = append(allComments, trailing)
	}
	return strings.Join(allComments, "\n")
}

// isInternalMethod 检查方法注释中是否包含"内部调用"或"不暴露 HTTP 端点"标记。
// 使用 collectMethodComments 统一收集全部注释源。
func isInternalMethod(m *protogen.Method) bool {
	return isInternalFromComments(collectMethodComments(m))
}

// isInternalFromComments 从已收集的注释字符串中判断是否为内部方法。
// 独立于 protogen，方便在已有注释文本的场景下避免重复收集。
func isInternalFromComments(combined string) bool {
	return strings.Contains(combined, "内部调用") || strings.Contains(combined, "不暴露 HTTP 端点")
}

func parseMethodComments(m *protogen.Method) (summary, description string) {
	return parseCommentsContent(collectMethodComments(m))
}

// parseCommentsContent 从合并后的注释字符串中提取 @summary 和 @description。
// 独立于 protogen，方便单元测试。
func parseCommentsContent(combined string) (summary, description string) {
	if combined == "" {
		return "", ""
	}

	lines := strings.Split(combined, "\n")
	var descLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "//")
		line = strings.TrimSpace(line)

		if match := summaryRegex.FindStringSubmatch(line); len(match) > 1 {
			summary = strings.TrimSpace(match[1])
			continue
		}

		if match := descriptionRegex.FindStringSubmatch(line); len(match) > 1 {
			descLines = append(descLines, strings.TrimSpace(match[1]))
		}
	}

	description = strings.Join(descLines, " ")
	return summary, description
}
