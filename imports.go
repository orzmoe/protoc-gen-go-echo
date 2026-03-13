package main

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"google.golang.org/protobuf/compiler/protogen"
)

// versionSegmentRe 匹配纯版本号路径段，如 v1、v2，不会误匹配 validator 等。
var versionSegmentRe = regexp.MustCompile(`^v\d+$`)

type fileImport struct {
	Alias string
	Path  string
}

type importManager struct {
	currentFileImportPath protogen.GoImportPath
	packageNames          map[protogen.GoImportPath]protogen.GoPackageName
	aliasesByPath         map[protogen.GoImportPath]string
	usedAliases           map[string]struct{}
}

func newImportManager(currentFileImportPath protogen.GoImportPath, packageNames map[protogen.GoImportPath]protogen.GoPackageName, reservedAliases ...string) *importManager {
	usedAliases := make(map[string]struct{}, len(reservedAliases))
	for _, alias := range reservedAliases {
		usedAliases[alias] = struct{}{}
	}
	return &importManager{
		currentFileImportPath: currentFileImportPath,
		packageNames:          packageNames,
		aliasesByPath:         make(map[protogen.GoImportPath]string),
		usedAliases:           usedAliases,
	}
}

func (m *importManager) QualifiedTypeName(msg *protogen.Message) string {
	if msg == nil {
		return ""
	}
	if isGoogleProtobufEmpty(msg) {
		return "emptypb.Empty"
	}
	if msg.GoIdent.GoImportPath == "" || msg.GoIdent.GoImportPath == m.currentFileImportPath {
		return msg.GoIdent.GoName
	}
	alias := m.aliasForImportPath(msg.GoIdent.GoImportPath)
	if alias == "" {
		return msg.GoIdent.GoName
	}
	return alias + "." + msg.GoIdent.GoName
}

func (m *importManager) aliasForImportPath(importPath protogen.GoImportPath) string {
	if importPath == "" || importPath == m.currentFileImportPath {
		return ""
	}
	if alias, exists := m.aliasesByPath[importPath]; exists {
		return alias
	}

	baseAlias := m.baseAlias(importPath)
	alias := baseAlias
	for suffix := 2; ; suffix++ {
		if _, exists := m.usedAliases[alias]; !exists {
			break
		}
		alias = fmt.Sprintf("%s%d", baseAlias, suffix)
	}

	m.aliasesByPath[importPath] = alias
	m.usedAliases[alias] = struct{}{}
	return alias
}

func (m *importManager) baseAlias(importPath protogen.GoImportPath) string {
	// 优先使用已知包名
	if packageName, exists := m.packageNames[importPath]; exists && packageName != "" {
		return sanitizeImportAlias(string(packageName))
	}
	// fallback: 基于最后路径段推导 alias
	parts := strings.Split(string(importPath), "/")
	lastPart := parts[len(parts)-1]
	// 如果末段是纯版本号（v1, v2 等），拼上前一段以获得更有意义的 alias
	if len(parts) >= 2 && versionSegmentRe.MatchString(lastPart) {
		return sanitizeImportAlias(parts[len(parts)-2] + lastPart)
	}
	alias := sanitizeImportAlias(lastPart)
	if alias == "" || alias == "pkg" {
		// 极端情况：末段清洗后无意义，尝试倒数第二段
		if len(parts) >= 2 {
			return sanitizeImportAlias(parts[len(parts)-2])
		}
		return "pkg"
	}
	return alias
}

func (m *importManager) Imports() []fileImport {
	imports := make([]fileImport, 0, len(m.aliasesByPath))
	for importPath, alias := range m.aliasesByPath {
		imports = append(imports, fileImport{
			Alias: alias,
			Path:  string(importPath),
		})
	}
	slices.SortFunc(imports, func(a, b fileImport) int {
		if a.Alias == b.Alias {
			return strings.Compare(a.Path, b.Path)
		}
		return strings.Compare(a.Alias, b.Alias)
	})
	return imports
}

func sanitizeImportAlias(alias string) string {
	if alias == "" {
		return "pkg"
	}

	var builder strings.Builder
	for i, r := range alias {
		switch {
		case r == '_', unicode.IsLetter(r):
			builder.WriteRune(r)
		case unicode.IsDigit(r):
			if i == 0 {
				builder.WriteByte('_')
			}
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}

	sanitized := builder.String()
	if sanitized == "" {
		return "pkg"
	}
	return sanitized
}
