package utils

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TypeName(f *protogen.Field) string {
	_, name := TypeConstructorBuilder(f)
	return name
}

// TypeConstructor 返回用 f 对应类型的默认值
func TypeConstructor(f *protogen.Field) string {
	ct, _ := TypeConstructorBuilder(f)
	return ct
}

// TypeConstructorBuilder 构建 f 对应类型的创建语句及名称
func TypeConstructorBuilder(f *protogen.Field) (constructor string, name string) {
	if f.Desc.IsList() {
		constructor = "make([]"
		name = "[]"
		if f.Message == nil { // primitive
			constructor += f.Desc.Kind().String()
			name += f.Desc.Kind().String()
		} else {
			constructor += "*" + f.Message.GoIdent.GoName // Structure Name
			name += f.Message.GoIdent.GoName
		}
		constructor += ", 0)"
	} else if f.Desc.IsMap() {
		key := TypeName(f.Message.Fields[0])
		value := TypeName(f.Message.Fields[1])
		constructor = fmt.Sprintf("make(map[%s]%s)", key, value)
		name = fmt.Sprintf("map[%s]%s", key, value)
	} else if f.Message != nil { // structure
		constructor = "nil"
		name = f.GoIdent.GoName
	} else { // primitive
		name = f.Desc.Kind().String()
		switch f.Desc.Kind() {
		case protoreflect.BoolKind:
			constructor = "false"
		case protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Sint32Kind,
			protoreflect.Sint64Kind, protoreflect.Fixed32Kind, protoreflect.Fixed64Kind,
			protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind,
			protoreflect.FloatKind, protoreflect.DoubleKind:
			constructor = "0"
		case protoreflect.StringKind:
			constructor = "\"\""
		case protoreflect.BytesKind:
			constructor = "nil"
		default:
			constructor = "UNSUPPORTED"
		}
	}

	return
}
