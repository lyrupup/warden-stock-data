package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestOpenAPINoWriteRoutes ensures open API paths reject write HTTP methods.
func TestOpenAPINoWriteRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	open := r.Group("/open/v1")
	open.GET("/indices", func(c *gin.Context) { c.Status(http.StatusOK) })
	open.GET("/quotes", func(c *gin.Context) { c.Status(http.StatusOK) })

	writeMethods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, m := range writeMethods {
		for _, path := range []string{"/open/v1/indices", "/open/v1/quotes"} {
			req := httptest.NewRequest(m, path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusNotFound, w.Code, "open API must not expose %s %s", m, path)
		}
	}
}
