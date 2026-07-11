package parse_go

import "testing"

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
