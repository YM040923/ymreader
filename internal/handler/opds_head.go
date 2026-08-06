package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type headResponseWriter struct {
	gin.ResponseWriter
}

func (w *headResponseWriter) Write(data []byte) (int, error) {
	return len(data), nil
}

func (w *headResponseWriter) WriteString(data string) (int, error) {
	return len(data), nil
}

func suppressOPDSHeadResponseBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodHead {
			c.Next()
			return
		}

		originalWriter := c.Writer
		c.Writer = &headResponseWriter{ResponseWriter: originalWriter}
		defer func() {
			c.Writer = originalWriter
		}()
		c.Next()
	}
}
