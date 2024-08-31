{{ define "RespFieldsTemplate" }}
    {{- if .Desc.IsList -}}
        {{ .GoName }}: {{ type_constructor . }}, // is list
    {{- else if .Desc.IsMap -}}
        {{ .GoName }}: {{ type_constructor . }}, // is map
    {{- else if .Message -}}
        {{ .GoName }}: &{{ type_name . }} {
            {{- range $index, $field := .Message.Fields -}}
                {{ template "RespFieldsTemplate" $field }}
            {{ end -}}
        },
    {{- else -}}
        {{ .GoName }}: {{ type_constructor . }},
    {{ end -}}
{{ end }}

{{ define "RespTemplate" }}
    resp := &{{ .GoIdent.GoName }} {
        {{- range $index, $field := .Fields -}}
            {{ template "RespFieldsTemplate" $field }}
        {{- end }}
    }
{{ end }}


{{ with $Method := . }}
{{ with $Name := $Method.ActionName -1 }}
{{ with $UpperName := camel $Name }}

type {{ $UpperName }}InjectArg struct {
    Args *{{ $Method.Args }}
    Resp *{{ $Method.Resp }}
    Tx go_biz_kit.Transaction
}

type {{ $UpperName }}InjectInterface = func(ctx context.Context, arg *{{ $UpperName}}InjectArg) (context.Context, error)

func default{{ $UpperName }}InjectMethod(ctx context.Context, _ *{{ $UpperName}}InjectArg) (context.Context, error) {
    return ctx, errors.ERROR_QUERY_NOT_IMPLEMENT.ToError("not implement query inject named {{ $UpperName }}")
}

{{- range $index, $action := $Method.Queries }}
{{ if $action.IsInject }}
var {{ $Name }}Inject{{ camel $action.InjectName }} {{ $UpperName }}InjectInterface = default{{ $UpperName }}InjectMethod
{{ end -}}
{{ end -}}

{{- range $index, $action := $Method.Queries }}
{{ if $action.IsInject }}
func Register{{ $UpperName }}InjectMethod{{ camel $action.InjectName }}(inj {{ $UpperName }}InjectInterface) {
    {{ $Name }}Inject{{ camel $action.InjectName }} = inj
}
{{ end -}}
{{ end -}}


type {{ $Name }}Interface interface {
	SQL(ctx context.Context) (context.Context, string)
	Args(ctx context.Context, args *{{ $Method.Args }}, resp *{{ $Method.Resp }}) (context.Context, interface{}, error)
	Dest(ctx context.Context, args *{{ $Method.Args }}, resp *{{ $Method.Resp }}, tx go_biz_kit.Transaction) (context.Context, error)
}

type {{ $Name }} struct {
    db go_biz_kit.Database
    actions []{{ $Method.ActionName -1 }}Interface
}

func new{{ $UpperName }}(db go_biz_kit.Database) *{{ $Name }} {
    return &{{ $Name }} {
        db: db,
        actions: []{{ $Name }}Interface {
            {{ range $index, $action := $Method.Queries -}}
                &{{ $Method.ActionName $index }}{},
            {{ end -}}
        },
    }
}

func (a *{{ $Name }}) String() string {
    return {{ camel $Method.OperationName }}
    // return "{{ $Name }}"
}

func (a *{{ $Name }}) ExecuteQuery(ctx context.Context, args interface{}) (interface{}, error) {
    argsImpl := args.(*{{ $Method.Args }})
    return a.executeQuery(ctx, argsImpl)
}

func (a *{{ $Name }}) executeQuery(ctx context.Context, args *{{ $Method.Args }}) (*{{ $Method.Resp }}, error) {
    {{ template "RespTemplate" $Method.Method.Output }}

    tx, ctx, err := a.db.GetTx(ctx)
    if !error1.IsEmptyError(err) {
        return nil, errors.ERROR_QUERY_DEST.FromErrorf(err, "error occur while getting transaction from db")
    }

    for index, action := range a.actions {
        var err error = nil
        ctx, err = action.Dest(ctx, args, resp, tx)
        if !error1.IsEmptyError(err)  {
            _ = tx.Get().Rollback()
            err = errors.ERROR_QUERY_DEST.FromErrorf(err, "error occur with query action %d", index)
            return nil, err
        }
    }

    err = tx.Get().Commit()
    return resp, errors.ERROR_QUERY_DEST.FromErrorf(err, "error occur while commit transaction")
}

{{ range $index, $action := $Method.Queries }}
    type {{ $Method.ActionName $index}} struct {
        db go_biz_kit.Database
    }

    func (a *{{ $Method.ActionName $index }}) SQL(ctx context.Context) (context.Context, string) {
        return ctx, "{{ $action.Query }}"
    }

    func (a *{{ $Method.ActionName $index }}) Args(ctx context.Context, _args *{{ $Method.Args }}, _resp *{{ $Method.Resp }}) (context.Context, interface{}, error) {
        {{ range $action.MergeArgs.Items -}}
            {{ if .IsContext }}
            _, is{{ camel .AssignPath }}Exists := ctx.Value("{{ .AssignPath }}").({{ .Type }})
            if !is{{ camel .AssignPath }}Exists {
                return ctx, nil, errors.ERROR_QUERY_ARGS_NOT_FOUND.ToError("extract args for action {{ $Method.ActionName $index }}, but {{ .AssignPath }} doesn't exists on context")
            }
            {{ end }}
        {{ end }}

        anno := struct {
            {{- range $action.MergeArgs.Items -}}
                {{ .Name }} {{ .Type }} `db:"{{.FullPath}}"`
            {{ end -}}
        } {
            {{ range $action.MergeArgs.Items -}}
                {{- if .IsContext -}}
                {{ .Name }}: ctx.Value("{{ .AssignPath }}").({{ .Type }}),
                {{- else -}}
                {{ .Name }}: {{ .AssignPath }},
                {{- end }}
            {{ end -}}
        }

        return ctx, &anno, nil
    }

    func (a *{{ $Method.ActionName $index }}) Dest(ctx context.Context, _args *{{ $Method.Args }}, _resp *{{ $Method.Resp }}, tx go_biz_kit.Transaction) (context.Context, error) {
        var err error = nil

        {{ if $action.IsEmptyAction }}
        ctx, _ = a.SQL(ctx)
        ctx, _, _ = a.Args(ctx, _args, _resp)
        {{ else if $action.IsInject }}
        ctx, _ = a.SQL(ctx)
        ctx, _, _ = a.Args(ctx, _args, _resp)
        {{ else }}
        ctx, sql := a.SQL(ctx)
        ctx, args, err := a.Args(ctx, _args, _resp)

        if err != nil {
            return ctx, errors.ERROR_QUERY_DEST.FromError(err)
        }

        {{ end }}

        {{- if $action.IsSelect -}}
            // select code
            var parsedSql string
            var parsedArgs []interface{}
            // rows, err := tx.Get().NamedQuery(sql, args)
            parsedSql, parsedArgs, err = tx.Get().BindNamed(sql, args)
            if !error1.IsEmptyError(err) {
                return ctx, errors.ERROR_QUERY_DEST.FromError(err)
            }

            rows, err := tx.Get().QueryxContext(ctx, parsedSql, parsedArgs...)
            if err != nil {
                return ctx, errors.ERROR_QUERY_DEST.FromError(err)
            }

            {{ range $action.MergeResp.Items }}
            {{ if .IsContext }}

            {{ end }}
            {{ end }}

            {{ range $index, $item := $action.MergeResp.Items }}
            {{ if $item.IsContext }}
                _item{{ $item.Name }}{{ $index }}Slice := make([]{{ .Type }}, 0)
            {{ end }}
            {{ end }}

            for rows.Next() {
                item := &struct {
                    {{- range $action.MergeResp.AnonymousItems -}}
                        {{ .Name }} {{ .Type }}
                    {{ end -}}
                } {}

                err = rows.StructScan(item)
                if !error1.IsEmptyError(err) {
                    return ctx, errors.ERROR_QUERY_DEST.FromError(err)
                }

                {{ range $index, $item := $action.MergeResp.Items }}
                   {{- if $item.IsContext }}
                        {{- if $item.Name }}
                        _item{{ $item.Name }}{{ $index }}Slice = append(_item{{ $item.Name }}{{ $index }}Slice, item.{{ $item.Name }})
                        {{ else }}
                        _item{{ $item.Name }}{{ $index }}Slice = append(_item{{ $item.Name }}{{ $index }}Slice, item)
                        {{ end }}
                    {{- else if .IsStruct -}}
                        {{ .AssignPath }} = append( {{ .AssignPath }}, &{{ .Type }} {
                            {{- range .FieldInfo.TypeInfo.StructFields -}}
                                {{ .Name }}: item.{{ .Name }},
                            {{- end -}}
                        })
                    {{ else -}}
                        {{ .AssignPath }} = append( {{ .AssignPath }}, item.{{ .Name }} )
                    {{ end -}}
                {{ end }}
            }

            {{ range $index, $item := $action.MergeResp.Items }}
            {{ if $item.IsContext }}
                ctx = context.WithValue(ctx, "{{ $item.AssignPath }}", _item{{ $item.Name }}{{ $index }}Slice)
            {{ end }}
            {{ end }}

            return ctx, nil

            // end of select code
        {{ end -}}

        {{ if $action.IsSelectOne }}
        // begin select one, if there has many data, we will return the first only
        // rows, err := tx.Get().NamedQuery(ctx, sql, args)
               var parsedSql string
                var parsedArgs []interface{}
                // rows, err := tx.Get().NamedQuery(sql, args)
                parsedSql, parsedArgs, err = tx.Get().BindNamed(sql, args)
                if !error1.IsEmptyError(err) {
                    return ctx, errors.ERROR_QUERY_DEST.FromError(err)
                }

                rows, err := tx.Get().QueryxContext(ctx, parsedSql, parsedArgs...)


        if !error1.IsEmptyError(err) {
            return ctx, errors.ERROR_QUERY_DEST.FromError(err)
        }

         for rows.Next() {
            item := &struct {
                {{- range $action.MergeResp.AnonymousItems -}}
                    {{ .Name }} {{ .Type }}
                {{ end -}}
            } {}

            err = rows.StructScan(item)
            if !error1.IsEmptyError(err) {
                return ctx, errors.ERROR_QUERY_DEST.FromError(err)
            }

            {{ range $action.MergeResp.Items -}}
                {{- if .IsContext }}
                    ctx = context.WithValue(ctx, "{{ .AssignPath }}", item.{{ .Name }})
                {{- else if .IsStruct -}}
                    {{ .AssignPath }} = &{{ .Type }} {
                        {{- range .FieldInfo.TypeInfo.StructFields -}}
                            {{ .Name }}: item.{{ .Name }},
                        {{- end -}}
                    }
                {{ else -}}
                    {{ .AssignPath }} = item.{{ .Name }}
                {{ end -}}
            {{ end }}
            break
        }

        return ctx, nil

        // end of select one
        {{ end }}

        {{ if $action.IsDIU }}
        // modify code
            {{ if $action.NoResponse -}}
            _, err = tx.Get().NamedExecContext(ctx, sql, args)
            {{ else -}}
            result, err := tx.Get().NamedExecContext(ctx, sql, args)
            {{ end }}
            if !error1.IsEmptyError(err) {
                return ctx, errors.ERROR_QUERY_DEST.FromError(err)
            }

            {{ if $action.MergeResp.Items }}
            rowsAffected, _ :=  result.RowsAffected()
            {{ end }}
            {{ range $action.MergeResp.Items -}}
                {{ if .IsContext }}
                    ctx = context.WithValue(ctx, "{{ .AssignPath }}", rowsAffected)
                {{ else }}
                    {{ .AssignPath }} = {{ .Type }}(rowsAffected)
                {{ end }}
            {{ end -}}
            return ctx, nil
        // end of insert code
        {{ end }}


        {{ if $action.IsInject }}
            ctx, err = {{ $Method.ActionName -1 }}Inject{{ camel $action.InjectName }}(ctx, &{{ $UpperName }}InjectArg {
                Args: _args,
                Resp: _resp,
                Tx: tx,
            })
            return ctx, errors.ERROR_QUERY_DEST.FromErrorf(err, "call query inject {{ camel $action.InjectName }} failed")
        {{ end }}


        {{ if $action.IsEmptyAction }}
            return ctx, nil
        {{ end }}
    }

{{ end }}

{{/* end with Name,UpperName,method */}}
{{ end }}
{{ end }}
{{ end }}