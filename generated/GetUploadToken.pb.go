// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v6.30.2
// source: .proto/GetUploadToken.proto

package generated

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type GetUploadToken struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	F1            int32                  `protobuf:"varint,1,opt,name=f1,proto3" json:"f1,omitempty"`
	F2            int32                  `protobuf:"varint,2,opt,name=f2,proto3" json:"f2,omitempty"`
	F3            int32                  `protobuf:"varint,3,opt,name=f3,proto3" json:"f3,omitempty"`
	F4            int32                  `protobuf:"varint,4,opt,name=f4,proto3" json:"f4,omitempty"`
	FileSizeBytes int64                  `protobuf:"varint,7,opt,name=file_size_bytes,json=fileSizeBytes,proto3" json:"file_size_bytes,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetUploadToken) Reset() {
	*x = GetUploadToken{}
	mi := &file___proto_GetUploadToken_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetUploadToken) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetUploadToken) ProtoMessage() {}

func (x *GetUploadToken) ProtoReflect() protoreflect.Message {
	mi := &file___proto_GetUploadToken_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetUploadToken.ProtoReflect.Descriptor instead.
func (*GetUploadToken) Descriptor() ([]byte, []int) {
	return file___proto_GetUploadToken_proto_rawDescGZIP(), []int{0}
}

func (x *GetUploadToken) GetF1() int32 {
	if x != nil {
		return x.F1
	}
	return 0
}

func (x *GetUploadToken) GetF2() int32 {
	if x != nil {
		return x.F2
	}
	return 0
}

func (x *GetUploadToken) GetF3() int32 {
	if x != nil {
		return x.F3
	}
	return 0
}

func (x *GetUploadToken) GetF4() int32 {
	if x != nil {
		return x.F4
	}
	return 0
}

func (x *GetUploadToken) GetFileSizeBytes() int64 {
	if x != nil {
		return x.FileSizeBytes
	}
	return 0
}

var File___proto_GetUploadToken_proto protoreflect.FileDescriptor

const file___proto_GetUploadToken_proto_rawDesc = "" +
	"\n" +
	"\x1b.proto/GetUploadToken.proto\"x\n" +
	"\x0eGetUploadToken\x12\x0e\n" +
	"\x02f1\x18\x01 \x01(\x05R\x02f1\x12\x0e\n" +
	"\x02f2\x18\x02 \x01(\x05R\x02f2\x12\x0e\n" +
	"\x02f3\x18\x03 \x01(\x05R\x02f3\x12\x0e\n" +
	"\x02f4\x18\x04 \x01(\x05R\x02f4\x12&\n" +
	"\x0ffile_size_bytes\x18\a \x01(\x03R\rfileSizeBytesb\x06proto3"

var (
	file___proto_GetUploadToken_proto_rawDescOnce sync.Once
	file___proto_GetUploadToken_proto_rawDescData []byte
)

func file___proto_GetUploadToken_proto_rawDescGZIP() []byte {
	file___proto_GetUploadToken_proto_rawDescOnce.Do(func() {
		file___proto_GetUploadToken_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file___proto_GetUploadToken_proto_rawDesc), len(file___proto_GetUploadToken_proto_rawDesc)))
	})
	return file___proto_GetUploadToken_proto_rawDescData
}

var file___proto_GetUploadToken_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file___proto_GetUploadToken_proto_goTypes = []any{
	(*GetUploadToken)(nil), // 0: GetUploadToken
}
var file___proto_GetUploadToken_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file___proto_GetUploadToken_proto_init() }
func file___proto_GetUploadToken_proto_init() {
	if File___proto_GetUploadToken_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file___proto_GetUploadToken_proto_rawDesc), len(file___proto_GetUploadToken_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file___proto_GetUploadToken_proto_goTypes,
		DependencyIndexes: file___proto_GetUploadToken_proto_depIdxs,
		MessageInfos:      file___proto_GetUploadToken_proto_msgTypes,
	}.Build()
	File___proto_GetUploadToken_proto = out.File
	file___proto_GetUploadToken_proto_goTypes = nil
	file___proto_GetUploadToken_proto_depIdxs = nil
}
