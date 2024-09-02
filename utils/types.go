package utils

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type FieldInfo struct {
	Field    *protogen.Field
	Name     string
	FullPath string
	Message  *protogen.Message
	TypeInfo *TypeInfo
}

type TypeInfo struct {
	IsArray       bool
	Type          string
	ArrayElemInfo *TypeInfo // array element info
	IsMap         bool
	KeyElemInfo   *TypeInfo
	ValueElemInfo *TypeInfo
	IsStruct      bool
	StructFields  []*FieldInfo
}

// ChooseFieldType 创建字段的类型信息
func ChooseFieldType(field *protogen.Field, basePath string) *TypeInfo {
	return &TypeInfo{
		Type:          field.Message.GoIdent.GoName,
		IsArray:       field.Desc.IsList(),
		ArrayElemInfo: nil,
		IsMap:         field.Desc.IsMap(),
		KeyElemInfo:   ChooseKeyType(field.Desc.MapKey()),
		ValueElemInfo: ChooseKeyType(field.Desc.MapValue()),
		IsStruct:      field.Message != nil,
		StructFields:  ChooseFields(field.Message, field.Message.Fields, basePath),
	}
}

// ChooseType 提取 message 中 fieldName 对应的字段，fieldName 是以 message 为起点的完整路径, 如 a.b.c.d
// fieldType 是在配置文件中指定的类型，如果传递了该参数，则以该参数的类型为准
func ChooseType(message *protogen.Message, fieldName string) *TypeInfo {
	namePath := strings.Split(fieldName, ".")
	if len(namePath) == 0 || message == nil {
		return nil
	}

	last := namePath[len(namePath)-1]
	namePath = namePath[:len(namePath)-1]
	//parentMessage := message
	fields := message.Fields
	var field *protogen.Field = nil

	for _, name := range namePath {
		for _, f := range fields {
			if f.GoName == name {
				field = f
				break
			}
		}

		if field == nil {
			panic(fmt.Sprintf("can't find field with path %s", fieldName))
		}

		if field.Message == nil {
			panic(fmt.Sprintf("field type doesn't match with path %s", fieldName))
		}

		//parentMessage = message
		message = field.Message
		fields = field.Message.Fields // next level
	}

	if last != "*" {
		newFields := make([]*protogen.Field, 0)
		for _, f := range fields {
			if f.GoName == CamelCase(last) {
				newFields = append(newFields, f)
				break
			}
		}

		fields = newFields
	}

	if len(fields) == 0 {
		panic(fmt.Sprintf("can't find field with path %s", fieldName))
	}

	//var arrayInfo *TypeInfo = nil
	//if field.Desc.IsList() {
	//	arrayInfo = ChooseType(parentMessage, field.GoName)
	//}

	if field == nil && len(fields) > 0 {
		field = fields[0]
	}

	typeName := field.Desc.Kind().String()
	if typeName == "message" {
		typeName = field.GoIdent.GoName
	}

	isArray := field.Desc.IsList()
	if isArray {
		// get array element type
		typeName = field.Desc.Kind().String()
	}

	return &TypeInfo{
		IsArray:       isArray,
		Type:          typeName, // 如果是 message 则要获取类型信息
		IsMap:         field.Desc.IsMap(),
		KeyElemInfo:   ChooseKeyType(field.Desc.MapKey()),
		ValueElemInfo: ChooseKeyType(field.Desc.MapValue()),
		IsStruct:      field.Message != nil,
		StructFields:  ChooseFields(message, fields, strings.Join(namePath, ".")),
	}
}

func ChooseKeyType(desc protoreflect.FieldDescriptor) *TypeInfo {
	// TODO
	return nil
}

func ChooseFields(message *protogen.Message, fields []*protogen.Field, basePath string) []*FieldInfo {
	fieldInfos := make([]*FieldInfo, 0)
	if len(fields) == 0 {
		return fieldInfos
	}

	for _, f := range fields {
		//var arrayInfo *TypeInfo = nil
		//if f.Desc.IsList() {
		//	arrayInfo = ChooseType(message, f.GoName)
		//}

		typeName := f.Desc.Kind().String()
		if typeName == "message" {
			typeName = f.GoIdent.GoName
		}

		isStruct := f.Message != nil
		isArray := f.Desc.IsList()
		if isArray {
			// get array element type
			typeName = f.Desc.Kind().String()
			if isStruct {
				typeName = f.Message.GoIdent.GoName
			}
		}

		innerFields := make([]*FieldInfo, 0)
		if isStruct {
			innerFields = ChooseFields(f.Message, f.Message.Fields, "")
		}

		info := &FieldInfo{
			Field:    f,
			Name:     f.GoName,
			FullPath: basePath + ".Get" + f.GoName + "()",
			Message:  message,
			TypeInfo: &TypeInfo{
				IsArray: f.Desc.IsList(),
				Type:    typeName,
				//ArrayElemInfo: arrayInfo,
				IsMap:         f.Desc.IsMap(),
				KeyElemInfo:   ChooseKeyType(f.Desc.MapKey()),
				ValueElemInfo: ChooseKeyType(f.Desc.MapValue()),
				IsStruct:      isStruct,
				StructFields:  innerFields,
			},
		}

		fieldInfos = append(fieldInfos, info)
	}
	return fieldInfos
}
