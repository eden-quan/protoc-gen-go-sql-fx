package utils

import (
	"fmt"

	def "github.com/eden-quan/protoc-gen-go-sql-fx/proto"
)

func ChooseAssignArgs(arg *def.DataBinding) string {
	a, _, _ := ChooseArgs(arg)
	return a
}

func ChooseAssignResp(arg *def.DataBinding) string {
	a, _, _ := ChooseResp(arg)
	return a
}

func ChooseArgs(arg *def.DataBinding) (string, string, error) {
	switch patten := arg.PattenFrom.(type) {
	case *def.DataBinding_FromQuery: // 该类型一般用于构建匿名类型，因此正常来说不会包含路径信息
		fromQuery := ConvertPath(patten.FromQuery)
		return fromQuery, fromQuery, nil
	case *def.DataBinding_FromArg:
		// from arg
		fromPath := ConvertPathWithGet(patten.FromArg)
		fromPath2 := ConvertPath(patten.FromArg)

		return "_args." + fromPath, fromPath2, nil
	case *def.DataBinding_FromResp:
		// from resp
		fromPath := ConvertPath(patten.FromResp)
		return "_resp." + fromPath, fromPath, nil
	case *def.DataBinding_FromContext:
		// from context
		return "ctx.Value(\"" + patten.FromContext + "\")", patten.FromContext, nil
	default:
		return "", "", fmt.Errorf("type of data binding mismatch: %v", arg.PattenFrom)
	}
}

func ChooseResp(arg *def.DataBinding) (string, string, error) {
	switch patten := arg.PattenTo.(type) {
	case *def.DataBinding_ToArg:
		toPath := ConvertPathWithGet(patten.ToArg)
		toPath2 := ConvertPath(patten.ToArg)

		return "_args." + toPath, toPath2, nil
	case *def.DataBinding_ToResp:
		toPath := ConvertPath(patten.ToResp)
		return "_resp." + toPath, toPath, nil
	case *def.DataBinding_ToContext:
		return "ctx.Value(\"" + patten.ToContext + "\")", patten.ToContext, nil
	default:
		return "", "", fmt.Errorf("type of data binding mismatch %v", arg.PattenTo)
	}
}
