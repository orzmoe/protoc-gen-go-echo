package main

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// 自定义 MethodOptions 扩展字段号（定义在 options.proto）
const (
	extFieldSetAuthCookie = 50001
	extFieldPermission    = 50002
	extFieldPermissions   = 50003
	extFieldAnyPermission = 50004
	extFieldPublic        = 50005
	extFieldAuthOnly      = 50006
	extFieldUpload        = 50007
	extFieldResponseMode  = 50008
)

// ResponseMode 定义显式响应模式，对应 proto enum。
type ResponseMode int32

const (
	ResponseModeUnspecified  ResponseMode = 0
	ResponseModeNormal       ResponseMode = 1
	ResponseModeFileDownload ResponseMode = 2
	ResponseModeRedirect     ResponseMode = 3
)

type methodOptions struct {
	setAuthCookie bool
	permission    string
	permissions   string
	anyPermission string
	public        bool
	authOnly      bool
	upload        bool
	responseMode  ResponseMode
}

func parseMethodOptions(m *protogen.Method) (methodOptions, error) {
	opts := m.Desc.Options()
	if opts == nil {
		return methodOptions{}, nil
	}

	optsProto, ok := opts.(*descriptorpb.MethodOptions)
	if !ok || optsProto == nil {
		return methodOptions{}, nil
	}

	unknownFields := proto.Message(optsProto).ProtoReflect().GetUnknown()
	var parsed methodOptions
	for len(unknownFields) > 0 {
		tag, n := protowire.ConsumeVarint(unknownFields)
		if n < 0 {
			return parsed, fmt.Errorf("解析扩展选项失败: 无法读取 field tag")
		}
		unknownFields = unknownFields[n:]

		fieldNumber := int32(tag >> 3)
		wireType := protowire.Type(tag & 0x7)

		switch wireType {
		case protowire.VarintType:
			val, n := protowire.ConsumeVarint(unknownFields)
			if n < 0 {
				return parsed, fmt.Errorf("解析扩展选项失败: field %d varint 损坏", fieldNumber)
			}
			unknownFields = unknownFields[n:]
			switch fieldNumber {
			case extFieldSetAuthCookie:
				parsed.setAuthCookie = val != 0
			case extFieldPublic:
				parsed.public = val != 0
			case extFieldAuthOnly:
				parsed.authOnly = val != 0
			case extFieldUpload:
				parsed.upload = val != 0
			case extFieldResponseMode:
				parsed.responseMode = ResponseMode(val)
			}
		case protowire.Fixed64Type:
			if len(unknownFields) < 8 {
				return parsed, fmt.Errorf("解析扩展选项失败: field %d fixed64 数据不足", fieldNumber)
			}
			unknownFields = unknownFields[8:]
		case protowire.BytesType:
			length, n := protowire.ConsumeVarint(unknownFields)
			if n < 0 {
				return parsed, fmt.Errorf("解析扩展选项失败: field %d 长度前缀损坏", fieldNumber)
			}
			unknownFields = unknownFields[n:]
			if length > uint64(len(unknownFields)) {
				return parsed, fmt.Errorf("解析扩展选项失败: field %d 声明长度 %d 超过剩余数据 %d", fieldNumber, length, len(unknownFields))
			}
			value := string(unknownFields[:int(length)])
			unknownFields = unknownFields[int(length):]
			switch fieldNumber {
			case extFieldPermission:
				parsed.permission = value
			case extFieldPermissions:
				parsed.permissions = value
			case extFieldAnyPermission:
				parsed.anyPermission = value
			}
		case protowire.Fixed32Type:
			if len(unknownFields) < 4 {
				return parsed, fmt.Errorf("解析扩展选项失败: field %d fixed32 数据不足", fieldNumber)
			}
			unknownFields = unknownFields[4:]
		default:
			return parsed, fmt.Errorf("解析扩展选项失败: field %d 未知 wire type %d", fieldNumber, wireType)
		}
	}
	return parsed, nil
}

func splitCommaSeparated(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// normalizeMethodOptions 对权限字符串做规范化：
// 1. 单值权限（permission）做 TrimSpace
// 2. 多值权限（permissions / anyPermission）用 splitCommaSeparated 解析后重新 join
// 3. 若原始值非空但规范化后为空，说明配置了纯空白内容，返回错误
func normalizeMethodOptions(opts *methodOptions) error {
	// 规范化 permission（单值）
	if opts.permission != "" {
		opts.permission = strings.TrimSpace(opts.permission)
		if opts.permission == "" {
			return fmt.Errorf("permission 值不能为纯空白")
		}
	}

	// 规范化 permissions（多值逗号分隔，去重）
	if opts.permissions != "" {
		parts := deduplicateStrings(splitCommaSeparated(opts.permissions))
		if len(parts) == 0 {
			return fmt.Errorf("permissions 值不能为纯空白或空列表")
		}
		opts.permissions = strings.Join(parts, ",")
	}

	// 规范化 anyPermission（多值逗号分隔，去重）
	if opts.anyPermission != "" {
		parts := deduplicateStrings(splitCommaSeparated(opts.anyPermission))
		if len(parts) == 0 {
			return fmt.Errorf("any_permission 值不能为纯空白或空列表")
		}
		opts.anyPermission = strings.Join(parts, ",")
	}

	return nil
}

// deduplicateStrings 保序去重字符串切片。
func deduplicateStrings(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(input))
	result := make([]string, 0, len(input))
	for _, s := range input {
		if _, exists := seen[s]; exists {
			continue
		}
		seen[s] = struct{}{}
		result = append(result, s)
	}
	return result
}

// validateMethodOptions 校验权限配置的互斥性。
// public / authOnly / permission / permissions / anyPermission 五者只允许一种语义生效。
// 注意：应在 normalizeMethodOptions 之后调用，此时权限值已完成规范化。
func validateMethodOptions(opts methodOptions) error {
	activeCount := 0
	var activeNames []string
	if opts.public {
		activeCount++
		activeNames = append(activeNames, "public")
	}
	if opts.authOnly {
		activeCount++
		activeNames = append(activeNames, "auth_only")
	}
	if opts.permission != "" {
		activeCount++
		activeNames = append(activeNames, "permission")
	}
	if opts.permissions != "" {
		activeCount++
		activeNames = append(activeNames, "permissions")
	}
	if opts.anyPermission != "" {
		activeCount++
		activeNames = append(activeNames, "any_permission")
	}
	if activeCount > 1 {
		return fmt.Errorf("权限选项冲突: 同时设置了 %s，只允许使用其中一个", strings.Join(activeNames, ", "))
	}

	// 校验 response_mode 值有效性
	switch opts.responseMode {
	case ResponseModeUnspecified, ResponseModeNormal, ResponseModeFileDownload, ResponseModeRedirect:
		// 合法值
	default:
		return fmt.Errorf("无效的 response_mode 值: %d", opts.responseMode)
	}

	return nil
}
