package main

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/eden-quan/protoc-gen-go-sql-fx/proto"
	"github.com/eden-quan/protoc-gen-go-sql-fx/utils"
)

type Query struct {
	*proto.DataMapping
	Args      []*DataBinding
	Resp      []*DataBinding
	Method    *protogen.Method
	MergeResp *MergeResp
	MergeArgs *MergeResp
}

type DataBinding struct {
	*proto.DataBinding
}

type MergeRespItem struct {
	Name       string           // 匿名类型的字段名
	Type       string           // 匿名类型的字段类型
	FullPath   string           // 字段来源的完整路径
	AssignPath string           // 字段赋值的路径，一般是 FullPath 记上参数前缀
	IsArray    bool             // 该字段是否数组，暂时无需使用
	IsStruct   bool             // 该字段是否结构体
	IsContext  bool             // 该字段是否对 context 进行操作
	FieldInfo  *utils.FieldInfo // 在用于赋值时，如果字段是结构体，则应该通过结构体的字段及类型名称创建新的实例
}

type MergeResp struct {
	Items          []*MergeRespItem // 涉及的字段，跟 Resp 的定义相同
	AnonymousItems []*MergeRespItem // 用于创建匿名类型的字段，从 Items 中合并相同项得到
}

func (q *Query) InitMergeArg(bindings []*DataBinding) *MergeResp {
	resp := &MergeResp{
		Items:          make([]*MergeRespItem, 0),
		AnonymousItems: make([]*MergeRespItem, 0),
	}

	for _, r := range bindings {
		if r.Name == "" {
			r.Name = "*" // 默认行为
		}
		assignName, name, err := utils.ChooseArgs(r.DataBinding)

		if !q.IsDIU() && err != nil {
			panic(err)
		}

		_, isContext := r.DataBinding.PattenFrom.(*proto.DataBinding_FromContext)

		if isContext {
			if r.Type == "" {
				panic(fmt.Errorf("args of query %s is need extract from context, but lost type info", q.Method.GoName))
			}

			resp.Items = append(resp.Items, &MergeRespItem{
				Name:       utils.CamelCase(r.Name),
				Type:       r.Type,
				FullPath:   r.Name,
				AssignPath: name,
				IsArray:    false,
				IsStruct:   false,
				IsContext:  isContext,
				FieldInfo:  nil,
			})
		} else {
			nameType := utils.ChooseType(q.Method.Input, name)

			targetField := nameType.StructFields[0]
			if r.Name != "*" {
				resp.Items = append(resp.Items, &MergeRespItem{
					Name:       utils.CamelCase(r.Name),
					Type:       targetField.TypeInfo.Type,
					FullPath:   r.Name,
					AssignPath: assignName,
					IsArray:    targetField.TypeInfo.IsArray,
					IsStruct:   targetField.TypeInfo.IsStruct,
					FieldInfo:  targetField,
				})
			} else if targetField.TypeInfo.IsStruct {
				for _, f := range targetField.TypeInfo.StructFields {
					resp.Items = append(resp.Items, &MergeRespItem{
						Name:       f.Name,
						Type:       f.TypeInfo.Type,
						FullPath:   string(f.Field.Desc.Name()),
						AssignPath: assignName + f.FullPath,
						IsArray:    f.TypeInfo.IsArray,
						IsStruct:   f.TypeInfo.IsStruct,
						FieldInfo:  f,
					})
				}
			}
		}
	}

	return resp
}

// InitMergeResp 合并所有 Resp 的结果信息，用于支持模板生成获取结果的匿名类型
// 几种 Query 的组合为：
// name -> a.b.c -> name=c
// name -> a     -> name=a 将 name 映射到 a
// * -> a.b.c    -> a.b.c.* 都要获取
func (q *Query) InitMergeResp(bindings []*DataBinding) *MergeResp {
	resp := MergeResp{
		Items:          make([]*MergeRespItem, 0),
		AnonymousItems: make([]*MergeRespItem, 0),
	}

	msgMap := make(map[string]string) // fieldName -> type, can use type to check conflict

	for _, r := range bindings {
		// 如果  name 不为 *, 则说明 name 与 nameType.fields[0] 是对应的
		// 如果 name 为 *，则说明 nameType.fields 中的每个字段都需要
		name, _, err := utils.ChooseArgs(r.DataBinding)
		if !q.IsDIU() && err != nil {
			panic(err)
		}

		//isContext := false
		_, isContext := r.PattenTo.(*proto.DataBinding_ToContext)
		isStruct := false
		isArray := false
		fieldType := ""
		toAssignOrigName, toOrigName, _ := utils.ChooseResp(r.DataBinding)
		var targetField *utils.FieldInfo = nil

		if isContext { // is context setting/gathering
			toAssignOrigName = fmt.Sprintf("ctx.set(\"%s\", %s", toOrigName, toOrigName)
			toAssignOrigName = toOrigName
			fieldType = r.Type
			isContext = true
		} else {
			nameType := utils.ChooseType(q.Method.Output, toOrigName)
			targetField = nameType.StructFields[0]
			fieldType = targetField.TypeInfo.Type
			isStruct = targetField.TypeInfo.IsStruct
			isArray = targetField.TypeInfo.IsArray

			// TODO: check type
			if name != "*" {
				if _, exists := msgMap[name]; !exists {
					resp.AnonymousItems = append(resp.AnonymousItems, &MergeRespItem{
						Name:       name,
						Type:       targetField.TypeInfo.Type,
						IsArray:    targetField.TypeInfo.IsArray,
						IsStruct:   targetField.TypeInfo.IsStruct,
						AssignPath: toAssignOrigName,
						FullPath:   toOrigName,
						FieldInfo:  targetField,
					})
					msgMap[name] = targetField.TypeInfo.Type
				}
			} else if targetField.TypeInfo.IsStruct {
				for _, f := range targetField.TypeInfo.StructFields {
					if _, exists := msgMap[f.Name]; !exists {
						resp.AnonymousItems = append(resp.AnonymousItems, &MergeRespItem{
							Name:      f.Name,
							Type:      f.TypeInfo.Type,
							FullPath:  f.FullPath,
							IsArray:   f.TypeInfo.IsArray,
							IsStruct:  f.TypeInfo.IsStruct,
							FieldInfo: f,
						})
						msgMap[f.Name] = f.TypeInfo.Type
					}
				}
			}
		}

		resp.Items = append(resp.Items, &MergeRespItem{
			Name:       name,
			Type:       fieldType,
			IsArray:    isArray,
			IsContext:  isContext,
			IsStruct:   isStruct,
			AssignPath: toAssignOrigName,
			FullPath:   toOrigName,
			FieldInfo:  targetField,
		})
	}
	return &resp
}

func (q *Query) IsSelect() bool {
	return q.Type == proto.QueryTypeEnum_Select
}

func (q *Query) IsSelectOne() bool {
	return q.Type == proto.QueryTypeEnum_SelectOne
}

func (q *Query) IsInject() bool {
	return q.Type == proto.QueryTypeEnum_Inject
}

func (q *Query) IsDIU() bool {
	return q.IsInsert() || q.IsUpdate() || q.IsDelete()
}

func (q *Query) IsUpdate() bool {
	return q.Type == proto.QueryTypeEnum_Update
}

func (q *Query) IsInsert() bool {
	return q.Type == proto.QueryTypeEnum_Insert
}

func (q *Query) IsDelete() bool {
	return q.Type == proto.QueryTypeEnum_Delete
}

func (q *Query) NoResponse() bool {
	return q.MergeResp == nil || len(q.MergeResp.Items) == 0
}

func (q *Query) NoArgs() bool {
	return len(q.Args) == 0
}

func (q *Query) IsEmptyAction() bool {
	return q.NoArgs() && q.NoResponse() && !q.IsInject()
}

func (d *DataBinding) FromIsContext() bool {
	_, ok := d.PattenFrom.(*proto.DataBinding_FromContext)
	return ok
}

func (d *DataBinding) ToIsContext() bool {
	_, ok := d.PattenTo.(*proto.DataBinding_ToContext)
	return ok
}
