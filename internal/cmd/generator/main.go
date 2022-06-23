package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"text/template"
)

//go:embed resources/bigquery-api.json
var bigqueryAPIJSON []byte

func main() {
	if err := run(os.Args); err != nil {
		log.Fatalf("%+v", err)
	}
}

type BigQueryAPI struct {
	Resources map[string]*Resource `json:"resources"`
}

type Resource struct {
	Methods map[string]*Method `json:"methods"`
}

type Method struct {
	HTTPMethod string                `json:"httpMethod"`
	Parameters map[string]*Parameter `json:"parameters"`
	Path       string                `json:"path"`
	FlatPath   string                `json:"flatPath"`
	Scope      []string              `json:"scope"`
}

type Parameter struct {
	Type     string `json:"type"`
	Location string `json:"location"`
	Required bool   `json:"required"`
}

type handlerParam struct {
	Path        string
	HTTPMethod  string
	HandlerName string
}

func run(args []string) error {
	var v BigQueryAPI
	if err := json.Unmarshal(bigqueryAPIJSON, &v); err != nil {
		return err
	}
	tmpl, err := template.New("").Parse(`// Code generated by internal/cmd/generator. DO NOT EDIT!
package server

import "net/http"

type Handler struct {
	Path       string
	HTTPMethod string
	Handler    http.Handler
}

var handlers = []*Handler{
{{- range . }}
	{
		Path: "{{ .Path }}",
		HTTPMethod: "{{ .HTTPMethod }}",
		Handler:    &{{ .HandlerName }}Handler{},
	},
{{- end }}
}

var (
{{- range . }}
  _ http.Handler = &{{ .HandlerName }}Handler{}
{{- end }}
)

{{- range . }}
type {{ .HandlerName }}Handler struct {
}
{{ end }}
`)
	handlerParams := []*handlerParam{}
	for resourceName, resource := range v.Resources {
		for methodName, method := range resource.Methods {
			path := method.FlatPath
			if path == "" {
				path = method.Path
			}
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			camelName := strings.ToUpper(string(methodName[0])) + methodName[1:]
			handlerParams = append(handlerParams, &handlerParam{
				Path:        path,
				HTTPMethod:  method.HTTPMethod,
				HandlerName: fmt.Sprintf("%s%s", resourceName, camelName),
			})
		}
	}
	sort.Slice(handlerParams, func(i, j int) bool {
		return handlerParams[i].HandlerName < handlerParams[j].HandlerName
	})
	var b bytes.Buffer
	if err := tmpl.Execute(&b, handlerParams); err != nil {
		return err
	}
	path := filepath.Join(repoRoot(), "server", "handler_gen.go")
	buf, err := format.Source(b.Bytes())
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, buf, 0644)
}

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	relativePathFromRepoRoot := filepath.Join("internal", "cmd", "generator")
	return strings.TrimSuffix(filepath.Dir(file), relativePathFromRepoRoot)
}
