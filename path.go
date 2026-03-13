package main

import (
	"fmt"
	"strings"
)

// normalizeEchoPath 将 google.api.http 路径模板归一化为 Echo 路由格式。
// 例如: /v1/items/{id} → /v1/items/:id, /v1/items/{id=*} → /v1/items/:id
func normalizeEchoPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	segments, err := splitPathSegments(path)
	if err != nil {
		return "", err
	}
	for i, segment := range segments {
		normalizedSegment, err := normalizeEchoPathSegment(segment)
		if err != nil {
			return "", err
		}
		segments[i] = normalizedSegment
	}
	return strings.Join(segments, "/"), nil
}

// splitPathSegments 按 '/' 切分路径，但保留花括号内的 '/' 不拆分。
// 例如: /v1/{name=projects/*/items/*} → ["", "v1", "{name=projects/*/items/*}"]
func splitPathSegments(path string) ([]string, error) {
	segments := make([]string, 0)
	var builder strings.Builder
	braceDepth := 0

	for _, r := range path {
		switch r {
		case '{':
			braceDepth++
			builder.WriteRune(r)
		case '}':
			braceDepth--
			if braceDepth < 0 {
				return nil, fmt.Errorf("非法的路径参数片段 %q", path)
			}
			builder.WriteRune(r)
		case '/':
			if braceDepth == 0 {
				segments = append(segments, builder.String())
				builder.Reset()
				continue
			}
			builder.WriteRune(r)
		default:
			builder.WriteRune(r)
		}
	}

	if braceDepth != 0 {
		return nil, fmt.Errorf("非法的路径参数片段 %q", path)
	}
	segments = append(segments, builder.String())
	return segments, nil
}

// normalizeEchoPathSegment 将单个路径段从 google.api.http 格式转换为 Echo 参数格式。
func normalizeEchoPathSegment(segment string) (string, error) {
	if segment == "" {
		return segment, nil
	}
	if strings.HasPrefix(segment, ":") {
		if len(segment) == 1 {
			return "", fmt.Errorf("非法的冒号路径参数 %q", segment)
		}
		return segment, nil
	}
	if !strings.HasPrefix(segment, "{") && !strings.HasSuffix(segment, "}") {
		return segment, nil
	}
	if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
		return "", fmt.Errorf("非法的路径参数片段 %q", segment)
	}

	inner := segment[1 : len(segment)-1]
	name, pattern, hasPattern := strings.Cut(inner, "=")
	if name == "" {
		return "", fmt.Errorf("非法的路径参数片段 %q", segment)
	}
	if !hasPattern || pattern == "*" {
		return ":" + name, nil
	}
	return "", fmt.Errorf("暂不支持复杂路径模板 %q", segment)
}
