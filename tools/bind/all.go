package bind

import "net/http"

type All struct {
	contentType string
}

func (All) Name() string {
	return "all"
}

func (a All) Bind(req *http.Request, obj interface{}) error {
	// 先判断 query & url & head 是否存在 尝试bind
	a.tryBind(req, obj)
	b := Default(a.contentType)
	return b.Bind(req, obj)
}

func (a All) tryBind(req *http.Request, obj interface{}) {
	a.tryBindQuery(req, obj)
	a.tryBindFormPost(req, obj)
	a.tryHand(req, obj)
}

// tryBindQuery 尝试绑定query
func (All) tryBindQuery(req *http.Request, obj interface{}) {
	values := req.URL.Query()
	if len(values) == 0 {
		return
	}
	_ = mapForm(obj, values)
}

func (All) tryBindFormPost(req *http.Request, obj interface{}) {
	if err := req.ParseForm(); err != nil {
		return
	}
	if err := req.ParseMultipartForm(defaultMemory); err != nil && err != http.ErrNotMultipart {
		return
	}
	if len(req.Form) == 0 {
		return
	}
	_ = mapForm(obj, req.Form)
}

func (All) tryHand(req *http.Request, obj interface{}) {
	if len(req.Header) != 0 {
		return
	}
	_ = mapHeader(obj, req.Header)
}

func CreateBindAll(contentType string) All {
	return All{contentType: contentType}
}
