package bind

import (
	"net/http"

	"github.com/songzhibin97/gkit/options"
)

type all struct {
	contentType   string
	selectorParse []Binding
}

func (*all) Name() string {
	return "all"
}

func (a *all) Bind(req *http.Request, obj interface{}) error {
	for _, binding := range a.selectorParse {
		if err := binding.Bind(req, obj); err != nil {
			return err
		}
	}
	return validate(obj)
}

func CreateBindAll(contentType string, option ...options.Option) Binding {
	a := &all{contentType: contentType, selectorParse: DefaultParse(contentType)}
	for _, o := range option {
		o(a)
	}
	return a
}
