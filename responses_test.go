package main

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

type testMessageField struct {
	name      string
	fieldType descriptorpb.FieldDescriptorProto_Type
}

func buildTestProtogenMessage(t *testing.T, messageName string, fields []testMessageField) *protogen.Message {
	t.Helper()

	msgFields := make([]*descriptorpb.FieldDescriptorProto, 0, len(fields))
	for i, field := range fields {
		msgFields = append(msgFields, &descriptorpb.FieldDescriptorProto{
			Name:   proto.String(field.name),
			Number: proto.Int32(int32(i + 1)),
			Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:   field.fieldType.Enum(),
		})
	}

	const fileName = "responses_unit_test.proto"
	request := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{fileName},
		ProtoFile: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String(fileName),
				Package: proto.String("test.v1"),
				Syntax:  proto.String("proto3"),
				Options: &descriptorpb.FileOptions{
					GoPackage: proto.String("example.com/test/v1;testv1"),
				},
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name:  proto.String(messageName),
						Field: msgFields,
					},
				},
			},
		},
	}

	plugin, err := (protogen.Options{}).New(request)
	if err != nil {
		t.Fatalf("构造 protogen 插件失败: %v", err)
	}
	if len(plugin.Files) != 1 {
		t.Fatalf("期望生成 1 个文件，实际 %d", len(plugin.Files))
	}

	file := plugin.Files[0]
	if len(file.Messages) != 1 {
		t.Fatalf("期望生成 1 个消息，实际 %d", len(file.Messages))
	}
	return file.Messages[0]
}

func assertErrorContainsAll(t *testing.T, err error, substrings []string) {
	t.Helper()
	if len(substrings) == 0 {
		if err != nil {
			t.Fatalf("期望错误为 nil，实际 %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("期望返回错误，实际为 nil")
	}
	for _, sub := range substrings {
		if !strings.Contains(err.Error(), sub) {
			t.Fatalf("错误信息不匹配，期望包含 %q，实际 %q", sub, err.Error())
		}
	}
}

func TestUnit_checkFileDownloadFields(t *testing.T) {
	tests := []struct {
		name   string
		fields []testMessageField
		want   fileDownloadFields
	}{
		{
			name: "完整字段",
			fields: []testMessageField{
				{name: "content", fieldType: descriptorpb.FieldDescriptorProto_TYPE_BYTES},
				{name: "filename", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
				{name: "content_type", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
			},
			want: fileDownloadFields{HasContent: true, HasFilename: true, HasContentType: true},
		},
		{
			name: "缺少部分字段",
			fields: []testMessageField{
				{name: "content", fieldType: descriptorpb.FieldDescriptorProto_TYPE_BYTES},
				{name: "filename", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
			},
			want: fileDownloadFields{HasContent: true, HasFilename: true, HasContentType: false},
		},
		{
			name: "字段类型错误",
			fields: []testMessageField{
				{name: "content", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
				{name: "filename", fieldType: descriptorpb.FieldDescriptorProto_TYPE_BYTES},
				{name: "content_type", fieldType: descriptorpb.FieldDescriptorProto_TYPE_INT32},
			},
			want: fileDownloadFields{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := buildTestProtogenMessage(t, "FileDownloadReply", tc.fields)
			got := checkFileDownloadFields(msg)
			if got != tc.want {
				t.Fatalf("字段检测结果不匹配，期望 %#v，实际 %#v", tc.want, got)
			}
		})
	}
}

func TestUnit_isFileDownloadResponse(t *testing.T) {
	tests := []struct {
		name   string
		fields []testMessageField
		want   bool
	}{
		{
			name: "正面_完整字段",
			fields: []testMessageField{
				{name: "content", fieldType: descriptorpb.FieldDescriptorProto_TYPE_BYTES},
				{name: "filename", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
				{name: "content_type", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
			},
			want: true,
		},
		{
			name: "负面_缺少字段",
			fields: []testMessageField{
				{name: "content", fieldType: descriptorpb.FieldDescriptorProto_TYPE_BYTES},
				{name: "filename", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
			},
			want: false,
		},
		{
			name: "负面_字段类型不匹配",
			fields: []testMessageField{
				{name: "content", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
				{name: "filename", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
				{name: "content_type", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := buildTestProtogenMessage(t, "IsFileDownloadResponseReply", tc.fields)
			got := isFileDownloadResponse(msg)
			if got != tc.want {
				t.Fatalf("文件下载响应判断不匹配，期望 %v，实际 %v", tc.want, got)
			}
		})
	}
}

func TestUnit_isRedirectResponse(t *testing.T) {
	tests := []struct {
		name   string
		fields []testMessageField
		want   bool
	}{
		{
			name: "单字段_redirect_url",
			fields: []testMessageField{
				{name: "redirect_url", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
			},
			want: true,
		},
		{
			name: "多字段",
			fields: []testMessageField{
				{name: "redirect_url", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
				{name: "extra", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
			},
			want: false,
		},
		{
			name: "字段名不对",
			fields: []testMessageField{
				{name: "url", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
			},
			want: false,
		},
		{
			name: "字段类型不对",
			fields: []testMessageField{
				{name: "redirect_url", fieldType: descriptorpb.FieldDescriptorProto_TYPE_BYTES},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := buildTestProtogenMessage(t, "RedirectReply", tc.fields)
			got := isRedirectResponse(msg)
			if got != tc.want {
				t.Fatalf("重定向响应判断不匹配，期望 %v，实际 %v", tc.want, got)
			}
		})
	}
}

func TestUnit_validateFileDownloadResponse(t *testing.T) {
	tests := []struct {
		name            string
		fields          []testMessageField
		wantErrContains []string
	}{
		{
			name: "缺少字段返回错误",
			fields: []testMessageField{
				{name: "content", fieldType: descriptorpb.FieldDescriptorProto_TYPE_BYTES},
			},
			wantErrContains: []string{"filename (string)", "content_type (string)"},
		},
		{
			name: "全部存在返回nil",
			fields: []testMessageField{
				{name: "content", fieldType: descriptorpb.FieldDescriptorProto_TYPE_BYTES},
				{name: "filename", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
				{name: "content_type", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
			},
			wantErrContains: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := buildTestProtogenMessage(t, "ValidateFileDownloadResponseReply", tc.fields)
			err := validateFileDownloadResponse(msg)
			assertErrorContainsAll(t, err, tc.wantErrContains)
		})
	}
}

func TestUnit_validateRedirectResponse(t *testing.T) {
	tests := []struct {
		name            string
		fields          []testMessageField
		wantErrContains []string
	}{
		{
			name: "存在redirect_url",
			fields: []testMessageField{
				{name: "redirect_url", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
			},
			wantErrContains: nil,
		},
		{
			name: "缺少redirect_url",
			fields: []testMessageField{
				{name: "url", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
			},
			wantErrContains: []string{"redirect_url (string)"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := buildTestProtogenMessage(t, "ValidateRedirectResponseReply", tc.fields)
			err := validateRedirectResponse(msg)
			assertErrorContainsAll(t, err, tc.wantErrContains)
		})
	}
}

func TestUnit_validateAuthCookieResponse(t *testing.T) {
	tests := []struct {
		name            string
		fields          []testMessageField
		wantErrContains []string
	}{
		{
			name: "缺失AccessToken",
			fields: []testMessageField{
				{name: "expires_in", fieldType: descriptorpb.FieldDescriptorProto_TYPE_INT64},
			},
			wantErrContains: []string{"access_token (string)"},
		},
		{
			name: "缺失ExpiresIn",
			fields: []testMessageField{
				{name: "access_token", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
			},
			wantErrContains: []string{"expires_in (int32/int64/uint32/uint64)"},
		},
		{
			name: "合法整数类型_int32",
			fields: []testMessageField{
				{name: "access_token", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
				{name: "expires_in", fieldType: descriptorpb.FieldDescriptorProto_TYPE_INT32},
			},
			wantErrContains: nil,
		},
		{
			name: "合法整数类型_int64",
			fields: []testMessageField{
				{name: "access_token", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
				{name: "expires_in", fieldType: descriptorpb.FieldDescriptorProto_TYPE_INT64},
			},
			wantErrContains: nil,
		},
		{
			name: "合法整数类型_uint32",
			fields: []testMessageField{
				{name: "access_token", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
				{name: "expires_in", fieldType: descriptorpb.FieldDescriptorProto_TYPE_UINT32},
			},
			wantErrContains: nil,
		},
		{
			name: "合法整数类型_uint64",
			fields: []testMessageField{
				{name: "access_token", fieldType: descriptorpb.FieldDescriptorProto_TYPE_STRING},
				{name: "expires_in", fieldType: descriptorpb.FieldDescriptorProto_TYPE_UINT64},
			},
			wantErrContains: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := buildTestProtogenMessage(t, "ValidateAuthCookieResponseReply", tc.fields)
			err := validateAuthCookieResponse(msg)
			assertErrorContainsAll(t, err, tc.wantErrContains)
		})
	}
}
