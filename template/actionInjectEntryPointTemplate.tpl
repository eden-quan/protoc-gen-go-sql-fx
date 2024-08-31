{{ with $Service := . }}
{{ with $Methods := $Service.Methods }}
{{ with $Name := $Service.ActionName }}
{{ with $UpperName := camel $Name }}
{{ if $Service.HasQuery }}

// Register{{ $UpperName }} 用于将当前 Service 的所有 SQL Action 注册到 IOC 容器中，便于公共库能够在适当的时候触发数据库操作
func Register{{ $UpperName }}() []interface{} {
    return []interface{} {
    {{ range $index, $method := $Methods }}
    {{ if $method.IsChain }}
    {{ with $MethodName := $method.ActionName -1 }}
        fx.Annotate(
            new{{ camel $MethodName }},
            fx.As(new(fmt.Stringer)),
            fx.ResultTags(`group:"sql_action_register"`),
        ),
    {{ end }}
    {{ end }}
    {{ end }}
    }
}

{{ end }}
{{ end }}
{{ end }}
{{ end }}
{{ end }}