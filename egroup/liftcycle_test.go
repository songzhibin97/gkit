package egroup

import (
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
		Start: func() error {
			t.Log("http start")
			srv.Handler = gin.Default()

			return srv.ListenAndServe()
		},
		Shutdown: func() {
			t.Log("http shutdown")
			srv.Shutdown(context.Background())
		},
	})
	fmt.Println("error", admin.Start())
	defer admin.shutdown()
}
