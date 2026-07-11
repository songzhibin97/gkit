package parse_go

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func parseDocFromGoSource(t *testing.T, source string) *Server {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "doc_tags.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Doc == nil {
			t.Fatal("function has no AST documentation comments")
		}
		docs := make([]string, len(fn.Doc.List))
		for i, comment := range fn.Doc.List {
			docs[i] = comment.Text
		}
		server := &Server{Doc: docs}
		parseDoc(server)
		return server
	}
	t.Fatal("source has no function declaration")
	return nil
}

func TestDocTagValueTrimsAndPreservesColons(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
		ok   bool
	}{
		{name: "trimmed", doc: "@method:  post  ", want: "post", ok: true},
		{name: "later colons", doc: "@router: /v1/users:lookup", want: "/v1/users:lookup", ok: true},
		{name: "empty", doc: "@service:   ", want: "", ok: true},
		{name: "missing colon", doc: "@method", want: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := docTagValue(tt.doc)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("docTagValue(%q) = (%q, %t), want (%q, %t)", tt.doc, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestParseDocAcceptsExactTagsAndRejectsEmbeddedText(t *testing.T) {
	server := &Server{Doc: []string{
		"   // @method:  post  ",
		"\t// @service: UserService ",
		" // @router: /v1/users:lookup ",
		"// ordinary text mentioning @method: delete",
		"// prefix @service: WrongService",
		"// prose with @router: /wrong",
		"// @methodology: put",
		"// @router : /also-wrong",
	}}

	parseDoc(server)

	if server.Method != "post" {
		t.Fatalf("Method = %q, want post", server.Method)
	}
	if server.ServerName != "UserService" {
		t.Fatalf("ServerName = %q, want UserService", server.ServerName)
	}
	if server.Router != "/v1/users:lookup" {
		t.Fatalf("Router = %q, want /v1/users:lookup", server.Router)
	}
}

func TestParseDocAcceptsBlockCommentPrefixes(t *testing.T) {
	server := &Server{Doc: []string{
		" /* @method: patch */ ",
		"/* @service: BlockService */",
		"/* @router: /block:route */",
	}}

	parseDoc(server)

	if server.Method != "patch" {
		t.Fatalf("Method = %q, want patch", server.Method)
	}
	if server.ServerName != "BlockService" {
		t.Fatalf("ServerName = %q, want BlockService", server.ServerName)
	}
	if server.Router != "/block:route" {
		t.Fatalf("Router = %q, want /block:route", server.Router)
	}
}

func TestParseDocAcceptsMultilineASTBlockComments(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		method      string
		serviceName string
		router      string
	}{
		{
			name: "leading stars",
			source: `package api

/*
 * @method: post
 * @service: UserService
 * @router: /v1/users:lookup
 */
func Handler() {}`,
			method:      "post",
			serviceName: "UserService",
			router:      "/v1/users:lookup",
		},
		{
			name: "without leading stars",
			source: `package api

/*
@method: get
@service: PlainService
@router: /plain:route
*/
func Handler() {}`,
			method:      "get",
			serviceName: "PlainService",
			router:      "/plain:route",
		},
		{
			name:        "CRLF and whitespace",
			source:      "package api\r\n\r\n/*\r\n\t *   @method:   patch   \r\n\t *\t@service:   CRLFService\t\r\n\t *   @router:   /crlf:route   \r\n\t */\r\nfunc Handler() {}\r\n",
			method:      "patch",
			serviceName: "CRLFService",
			router:      "/crlf:route",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := parseDocFromGoSource(t, tt.source)
			if server.Method != tt.method || server.ServerName != tt.serviceName || server.Router != tt.router {
				t.Fatalf(
					"parsed tags = Method %q, ServerName %q, Router %q; want %q, %q, %q",
					server.Method,
					server.ServerName,
					server.Router,
					tt.method,
					tt.serviceName,
					tt.router,
				)
			}
		})
	}
}

func TestParseDocRejectsEmbeddedAndNearMatchTagsInASTBlockComment(t *testing.T) {
	server := parseDocFromGoSource(t, `package api

/*
 * prose mentioning @method: delete
 * prefix @service: WrongService
 * prose with @router: /wrong
 * @methodology: put
 * @service : AlsoWrong
 * @router : /also-wrong
 */
func Handler() {}`)

	if server.Method != "" || server.ServerName != "" || server.Router != "" {
		t.Fatalf(
			"near matches parsed as Method=%q ServerName=%q Router=%q, want all empty",
			server.Method,
			server.ServerName,
			server.Router,
		)
	}
}

func TestParseDocAcceptsEmptyValues(t *testing.T) {
	server := &Server{
		Method:     "keep-method",
		ServerName: "keep-service",
		Router:     "keep-router",
		Doc: []string{
			"// @method:   ",
			"// @service:\t",
			"// @router: ",
		},
	}

	parseDoc(server)

	if server.Method != "" || server.ServerName != "" || server.Router != "" {
		t.Fatalf(
			"empty tags parsed as Method=%q ServerName=%q Router=%q, want all empty",
			server.Method,
			server.ServerName,
			server.Router,
		)
	}
}
