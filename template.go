package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/eden-quan/protoc-gen-go-sql-fx/proto"
	"github.com/eden-quan/protoc-gen-go-sql-fx/utils"
)

//go:embed template/actionDefTemplate.tpl
var actionSqlDefTemplate string

//go:embed template/actionInjectEntryPointTemplate.tpl
var actionInjectEntrypointTemplate string

type ServiceDesc struct {
	ServiceType      string
	ServiceName      string
	ServiceShortName string
	MetaData         string
	Methods          []MethodDesc
}

func (s *ServiceDesc) Execute() string {
	//templates := []string{actionSqlTemplate, actionArgsTemplate, actionDestTemplate, actionImplTemplate}
	generated := ""
	templates := []string{actionSqlDefTemplate}
	for _, m := range s.Methods {
		tpl := make([]string, 0, 4)
		for _, action := range templates {
			tmp, err := s.renderMethod(action, &m)
			if err != nil {
				panic(fmt.Sprintf("renderMethod failed with error %s", err))
			}

			tpl = append(tpl, tmp)
			tpl = append(tpl, "\r\n")
		}

		generated = generated + strings.Join(tpl, "\r\n")
	}

	globalTmp, err := s.render(actionInjectEntrypointTemplate, s)
	if err == nil {
		generated += globalTmp
	}

	// generate inject all entrypoint
	return generated
}

// registerFunc should call before t's parse
func (s *ServiceDesc) registerFunc(t *template.Template) *template.Template {
	funcMap := map[string]any{
		"camel":            utils.CamelCase,
		"choose_args":      utils.ChooseAssignArgs,
		"choose_resp":      utils.ChooseAssignResp,
		"type_constructor": utils.TypeConstructor,
		"type_name":        utils.TypeName,
	}

	return t.Funcs(funcMap)
}

func (s *ServiceDesc) render(tpl string, args interface{}) (string, error) {
	tmpl := template.New("tpl")
	tmpl = s.registerFunc(tmpl)
	tmpl, err := tmpl.Parse(strings.TrimSpace(tpl))
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, args)

	return strings.Trim(buf.String(), "\r\n"), err
}

func (s *ServiceDesc) renderMethod(tpl string, method *MethodDesc) (string, error) {

	if len(method.Queries) == 0 {
		return "", nil
	}

	return s.render(tpl, method)
}

func (s *ServiceDesc) ActionName() string {
	return fmt.Sprintf("%sSQLAction", s.ServiceShortName)
}

func (s *ServiceDesc) HasQuery() bool {
	for _, m := range s.Methods {
		if m.IsSQL() {
			return true
		}
	}

	return false
}

type MethodDesc struct {
	Service        *ServiceDesc
	MethodName     string
	OriginalName   string
	Args           string
	Resp           string
	Comment        string
	Query          *proto.DataQuery
	Chain          *proto.QueryChain
	Queries        []*Query // build from query/chain
	Method         *protogen.Method
	ActionNameBase string
}

func (m *MethodDesc) Init() {
	if m.IsQuery() {
		m.Chain = &proto.QueryChain{
			Query: []*proto.DataMapping{
				m.Query.Query,
			},
		}
	}

	if !m.IsChain() {
		return
	}

	m.Queries = make([]*Query, 0)
	for _, q := range m.Chain.Query {
		args := make([]*DataBinding, 0)
		resp := make([]*DataBinding, 0)
		for _, a := range q.Args {
			args = append(args, &DataBinding{a})
		}

		if q.Type == proto.QueryTypeEnum_QUERY_UNSPECIFIED {
			q.Type = proto.QueryTypeEnum_Select
		}

		for _, r := range q.Resp {
			resp = append(resp, &DataBinding{r})
		}

		q := &Query{
			DataMapping: q,
			Args:        args,
			Resp:        resp,
			Method:      m.Method,
		}

		mergedResp := q.InitMergeResp(q.Resp)
		q.MergeResp = mergedResp
		mergedArgs := q.InitMergeArg(q.Args)
		q.MergeArgs = mergedArgs
		m.Queries = append(m.Queries, q)
	}
}

func (m *MethodDesc) ActionName(index int) string {
	if index > -1 {
		return fmt.Sprintf("%s%sSQLAction%d", utils.Unexport(m.Service.ServiceShortName), m.MethodName, index)
	} else {
		return fmt.Sprintf("%s%sSQLAction", m.Service.ServiceShortName, m.MethodName)
	}
}

func (m *MethodDesc) OperationName() string {
	return fmt.Sprintf("Operation%s%s", utils.CamelCase(m.Service.ServiceShortName), m.MethodName)
}

func (m *MethodDesc) IsQuery() bool {
	return m.Query != nil
}

func (m *MethodDesc) IsChain() bool {
	return m.Chain != nil && len(m.Chain.Query) > 0
}

func (m *MethodDesc) IsSQL() bool {
	return m.IsQuery() || m.IsChain()
}
