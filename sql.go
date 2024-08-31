package main

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"

	def "github.com/eden-quan/protoc-gen-go-sql-fx/proto"
	"github.com/eden-quan/protoc-gen-go-sql-fx/utils"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	contextPackage       = protogen.GoImportPath("context")
	fmtPackage           = protogen.GoImportPath("fmt")
	fxPackage            = protogen.GoImportPath("go.uber.org/fx")
	transportHTTPPackage = protogen.GoImportPath("github.com/go-kratos/kratos/v2/transport/http")
	bindingPackage       = protogen.GoImportPath("github.com/go-kratos/kratos/v2/transport/http/binding")
	businessPackage      = protogen.GoImportPath("github.com/eden-quan/go-biz-kit")
	businessErrorPackage = protogen.GoImportPath("github.com/eden-quan/go-biz-kit/error")
)

// generateFile generates a _http.pb.go file containing kratos errors definitions.
func generateFile(gen *protogen.Plugin, file *protogen.File, omitempty bool, omitemptyPrefix string) *protogen.GeneratedFile {
	if len(file.Services) == 0 {
		return nil
	}

	filename := file.GeneratedFilenamePrefix + "_sql.pb.go"
	g := gen.NewGeneratedFile(filename, file.GoImportPath)
	g.P("// Code generated by protoc-gen-go-sql-fx. DO NOT EDIT.")
	g.P("// versions:")
	g.P(fmt.Sprintf("// - protoc-gen-go-sql-fx %s", version))
	g.P("// - protoc             ", utils.ProtocVersion(gen))
	if file.Proto.GetOptions().GetDeprecated() {
		g.P("// ", file.Desc.Path(), " is a deprecated file.")
	} else {
		g.P("// source: ", file.Desc.Path())
	}
	g.P()
	g.P("package ", file.GoPackageName)
	g.P()
	generateFileContent(gen, file, g, omitempty, omitemptyPrefix)
	return g
}

// generateFileContent generates the kratos errors definitions, excluding the package statement.
func generateFileContent(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, omitempty bool, omitemptyPrefix string) {
	if len(file.Services) == 0 {
		return
	}
	g.P("// This is a compile-time assertion to ensure that this generated file")
	g.P("// is compatible with the kratos package it is being compiled against.")
	g.P("var _ = new(", contextPackage.Ident("Context"), ")")
	g.P("var _ = new(", fmtPackage.Ident("Stringer"), ")")
	g.P("var _ = new(", businessPackage.Ident("Database"), ")")
	g.P("var _ = ", bindingPackage.Ident("EncodeURL"))
	g.P("const _ = ", transportHTTPPackage.Ident("SupportPackageIsVersion1"))
	g.P("const _ = ", fxPackage.Ident("Version"))
	g.P("var _ = new(", businessErrorPackage.Ident("TruncateToEmptyError"), ")")

	for _, service := range file.Services {
		genService(gen, file, g, service, omitempty, omitemptyPrefix)
	}
}

func genService(p *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, service *protogen.Service, omitempty bool, omitemptyPrefix string) {
	if service.Desc.Options().(*descriptorpb.ServiceOptions).GetDeprecated() {
		g.P("//")
		g.P(utils.DeprecationComment)
	}

	// HTTP Server.
	sd := &ServiceDesc{
		ServiceType:      service.GoName,
		ServiceName:      string(service.Desc.FullName()),
		ServiceShortName: utils.Unexport(service.GoName),
		MetaData:         file.Desc.Path(),
	}

	for _, method := range service.Methods {
		if method.Desc.IsStreamingClient() || method.Desc.IsStreamingServer() {
			continue
		}
		crud, _ := proto.GetExtension(method.Desc.Options(), def.E_Crud).(*def.DataQuery)
		chain, _ := proto.GetExtension(method.Desc.Options(), def.E_Chain).(*def.QueryChain)
		comment := method.Comments.Leading.String() + method.Comments.Trailing.String()
		if comment != "" {
			comment = "// " + method.GoName + strings.TrimPrefix(strings.TrimSuffix(comment, "\n"), "//")
		}

		method = FlattenFilter(method)
		// 过滤 Method 的 Flatten 字段

		m := MethodDesc{
			Service:    sd,
			MethodName: method.GoName,
			Method:     method,
			Query:      crud,
			Chain:      chain,
			Comment:    comment,
			Args:       g.QualifiedGoIdent(method.Input.GoIdent),
			Resp:       g.QualifiedGoIdent(method.Output.GoIdent),
		}

		m.Init()
		//if len(m.Queries) > 0 {
		//	m.Queries[0].MergeResp()
		//}
		sd.Methods = append(sd.Methods, m)
	}

	if len(sd.Methods) != 0 {
		g.P(sd.Execute())
	}
}

func extractFlattenDesc(msg protoreflect.ProtoMessage) bool {
	flatten := proto.GetExtension(msg, def.E_Flatten).(bool)
	//ext := proto.GetExtension(msg, def.E_FlattenRule).(*def.FlattenRules)
	return flatten
}

func FlattenFilter(method *protogen.Method) *protogen.Method {
	fields := make([]*protogen.Field, 0)

	for _, f := range method.Output.Fields {
		if f.Message != nil {
			flatten := proto.GetExtension(f.Desc.Options(), def.E_Flatten).(bool)

			if flatten {
				continue
			}
		}

		fields = append(fields, f)
	}
	method.Output.Fields = fields
	return method
}
