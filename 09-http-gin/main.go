// Example 09-http-gin — graceful shutdown of a Gin engine wrapped in *http.Server.
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ubgo/shutdown"
	shutdowngin "github.com/ubgo/shutdown/contrib/shutdown-gin"
)

func main() {
	mgr := shutdown.New(shutdown.WithBudget(15 * time.Second))

	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		time.Sleep(2 * time.Second)
		c.String(http.StatusOK, "hello")
	})
	srv := &http.Server{Addr: ":8080", Handler: r}

	if err := shutdowngin.Register(mgr, srv); err != nil {
		panic(err)
	}

	go func() {
		fmt.Println("listening on :8080 — Ctrl-C to shut down")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("listen:", err)
		}
	}()

	if err := mgr.Listen(context.Background()); err != nil {
		fmt.Println("shutdown:", err)
	}
}
