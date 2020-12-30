package egroup

import (
	"Songzhibin/GKit/goroutine"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"testing"
)

var admin = NewLifeAdmin()

func TestLifeAdmin_Start(t *testing.T) {
	srv := &http.Server{
		Addr: ":8080",
	}
	admin.Add(Member{
		Start: func(ctx context.Context) error {
			t.Log("http start")
			srv.Handler = gin.Default()
			return goroutine.Delegate(ctx,-1, func(ctx context.Context) error {
				return srv.ListenAndServe()
			})
		},
		Shutdown: func(ctx context.Context) error {
			t.Log("http shutdown")
			return srv.Shutdown(context.Background())
		},
	})
	fmt.Println("error", admin.Start())
	defer admin.shutdown()
}
