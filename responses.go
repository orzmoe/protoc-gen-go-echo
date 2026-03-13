package main

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func isGoogleProtobufEmpty(msg *protogen.Message) bool {
	if msg == nil {
		return false
	}
	return string(msg.Desc.FullName()) == "google.protobuf.Empty"
}

// fileDownloadFields 描述文件下载响应消息的三个必需字段的检测结果。
type fileDownloadFields struct {
	HasContent     bool
	HasFilename    bool
	HasContentType bool
}

// checkFileDownloadFields 检测响应消息中是否包含文件下载所需的三个字段。
func checkFileDownloadFields(msg *protogen.Message) fileDownloadFields {
	var f fileDownloadFields
	for _, field := range msg.Fields {
		switch field.GoName {
		case "Content":
			f.HasContent = field.Desc.Kind() == protoreflect.BytesKind
		case "Filename":
			f.HasFilename = field.Desc.Kind() == protoreflect.StringKind
		case "ContentType":
			f.HasContentType = field.Desc.Kind() == protoreflect.StringKind
		}
	}
	return f
}

func (f fileDownloadFields) allPresent() bool {
	return f.HasContent && f.HasFilename && f.HasContentType
}

func (f fileDownloadFields) missingNames() []string {
	var missing []string
	if !f.HasContent {
		missing = append(missing, "content (bytes)")
	}
	if !f.HasFilename {
		missing = append(missing, "filename (string)")
	}
	if !f.HasContentType {
		missing = append(missing, "content_type (string)")
	}
	return missing
}

// isFileDownloadResponse 检测响应类型是否为文件下载
// 通过检查是否包含 Content(bytes)、Filename(string)、ContentType(string) 三个字段
func isFileDownloadResponse(msg *protogen.Message) bool {
	return checkFileDownloadFields(msg).allPresent()
}

// validateFileDownloadResponse 校验 response_mode=FILE_DOWNLOAD 时响应消息结构。
func validateFileDownloadResponse(msg *protogen.Message) error {
	if missing := checkFileDownloadFields(msg).missingNames(); len(missing) > 0 {
		return fmt.Errorf("响应消息缺少必需字段: %s", strings.Join(missing, ", "))
	}
	return nil
}

// isRedirectResponse 检测响应类型是否为重定向
// 通过检查是否只包含一个 RedirectUrl(string) 字段
func isRedirectResponse(msg *protogen.Message) bool {
	if len(msg.Fields) != 1 {
		return false
	}

	field := msg.Fields[0]
	return field.GoName == "RedirectUrl" && field.Desc.Kind() == protoreflect.StringKind
}

// validateRedirectResponse 校验 response_mode=REDIRECT 时响应消息结构。
func validateRedirectResponse(msg *protogen.Message) error {
	for _, field := range msg.Fields {
		if field.GoName == "RedirectUrl" && field.Desc.Kind() == protoreflect.StringKind {
			return nil
		}
	}
	return fmt.Errorf("响应消息缺少必需字段: redirect_url (string)")
}

// validateAuthCookieResponse 校验 set_auth_cookie 的响应消息必须包含:
// - AccessToken (string): 用于 cookie 值
// - ExpiresIn (int32/int64/uint32/uint64): 用于 cookie MaxAge
func validateAuthCookieResponse(msg *protogen.Message) error {
	hasAccessToken := false
	hasExpiresIn := false

	for _, field := range msg.Fields {
		switch field.GoName {
		case "AccessToken":
			if field.Desc.Kind() == protoreflect.StringKind {
				hasAccessToken = true
			}
		case "ExpiresIn":
			kind := field.Desc.Kind()
			if kind == protoreflect.Int32Kind || kind == protoreflect.Int64Kind ||
				kind == protoreflect.Uint32Kind || kind == protoreflect.Uint64Kind {
				hasExpiresIn = true
			}
		}
	}

	var missing []string
	if !hasAccessToken {
		missing = append(missing, "access_token (string)")
	}
	if !hasExpiresIn {
		missing = append(missing, "expires_in (int32/int64/uint32/uint64)")
	}
	if len(missing) > 0 {
		return fmt.Errorf("set_auth_cookie 要求响应消息包含以下字段: %s", strings.Join(missing, ", "))
	}
	return nil
}
