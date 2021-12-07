package egroup

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/songzhibin97/gkit/goroutine"
)

var _admin = NewLifeAdmin()

func TestLifeAdmin_Start(t *testing.T) {
	srv := &http.Server{
		Addr: ":8080",
	}
	_admin.Add(Member{
		Start: func(ctx context.Context) error {
			t.Log("http start")
			return goroutine.Delegate(ctx, -1, func(ctx context.Context) error {
				return srv.ListenAndServe()
			})
		},
		Shutdown: func(ctx context.Context) error {
			t.Log("http shutdown")
			return srv.Shutdown(context.Background())
		},
	})
	//_admin.Add(Member{
	//	Start: func(ctx context.Context) error {
	//		time.Sleep(5 * time.Second)
	//		t.Log("error")
	//		return errors.New("error")
	//	},
	//})
	fmt.Println("error", _admin.Start())
	defer _admin.shutdown()
}
