package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSecretShieldMiddleware_SanitizesRequestAndRestoresResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var bodySeenByHandler string
	r.POST("/v1/responses",
		func(c *gin.Context) {
			c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{SecretShieldEnabled: true}})
			c.Next()
		},
		SecretShieldMiddleware(),
		func(c *gin.Context) {
			body, err := io.ReadAll(c.Request.Body)
			require.NoError(t, err)
			bodySeenByHandler = string(body)
			require.NotContains(t, bodySeenByHandler, "sk-proj-abcdefghijklmnopqrstuvwxyz123456")
			require.Contains(t, bodySeenByHandler, secretShieldPlaceholderPrefixForTest())

			idx := strings.Index(bodySeenByHandler, secretShieldPlaceholderPrefixForTest())
			require.GreaterOrEqual(t, idx, 0)
			split := idx + len(secretShieldPlaceholderPrefixForTest()) + 3
			c.Header("Content-Length", "9999")
			_, _ = c.Writer.Write([]byte(bodySeenByHandler[:split]))
			_, _ = c.Writer.Write([]byte(bodySeenByHandler[split:]))
		},
	)

	original := `{"model":"gpt-5.5","input":"OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz123456"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(original))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, bodySeenByHandler, "sk-proj-abcdefghijklmnopqrstuvwxyz123456")
	require.Equal(t, original, rec.Body.String())
	require.Empty(t, rec.Header().Get("Content-Length"))
}

func TestSecretShieldMiddleware_DisabledLeavesRequestAndResponseUntouched(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/v1/responses",
		func(c *gin.Context) {
			c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{SecretShieldEnabled: false}})
			c.Next()
		},
		SecretShieldMiddleware(),
		func(c *gin.Context) {
			body, err := io.ReadAll(c.Request.Body)
			require.NoError(t, err)
			_, _ = c.Writer.Write(body)
		},
	)

	original := `{"model":"gpt-5.5","input":"OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz123456"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(original))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, original, rec.Body.String())
}

func TestSecretShieldMiddleware_RestoresWriterBeforeOuterMiddlewareContinues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var statusSeenByOuter int
	r.POST("/v1/responses",
		func(c *gin.Context) {
			c.Next()
			statusSeenByOuter = c.Writer.Status()
		},
		func(c *gin.Context) {
			c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{SecretShieldEnabled: true}})
			c.Next()
		},
		OpsErrorLoggerMiddleware(nil),
		SecretShieldMiddleware(),
		func(c *gin.Context) {
			body, err := io.ReadAll(c.Request.Body)
			require.NoError(t, err)
			require.Contains(t, string(body), secretShieldPlaceholderPrefixForTest())
			_, _ = c.Writer.Write(body)
		},
	)

	original := `{"model":"gpt-5.5","input":"OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz123456"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(original))
	req.Header.Set("Content-Type", "application/json")

	require.NotPanics(t, func() {
		r.ServeHTTP(rec, req)
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, http.StatusOK, statusSeenByOuter)
	require.Equal(t, original, rec.Body.String())
}

func secretShieldPlaceholderPrefixForTest() string {
	return "__S2A_SECRET_"
}
