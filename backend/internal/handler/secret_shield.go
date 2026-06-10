package handler

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const secretShieldWriterContextKey = "secret_shield_response_writer"

func secretShieldEnabled(apiKey *service.APIKey) bool {
	return apiKey != nil && apiKey.Group != nil && apiKey.Group.SecretShieldEnabled
}

// SecretShieldMiddleware protects request-local secrets before any downstream
// moderation/upstream forwarding sees the body, then restores known placeholders
// only in successful client responses. The vault is scoped to this request.
func SecretShieldMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey, ok := middleware2.GetAPIKeyFromContext(c)
		if !ok || !secretShieldEnabled(apiKey) || !secretShieldRequestMayHaveBody(c) || secretShieldSkipContentType(c.GetHeader("Content-Type")) {
			c.Next()
			return
		}

		body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			status := http.StatusBadRequest
			message := "Failed to read request body"
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				status = http.StatusRequestEntityTooLarge
				message = buildBodyTooLargeMessage(maxErr.Limit)
			}
			c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"type": "invalid_request_error", "message": message}})
			return
		}
		if len(body) == 0 {
			resetSecretShieldRequestBody(c, body)
			c.Next()
			return
		}

		vault := service.NewSecretShieldVault()
		protected, changed := vault.ProtectBytes(body)
		resetSecretShieldRequestBody(c, protected)
		if !changed {
			c.Next()
			return
		}

		writer := installSecretShieldResponseWriter(c, vault)
		defer func() {
			_ = writer.Finalize()
			if writer != nil && c.Writer == writer {
				c.Writer = writer.ResponseWriter
			}
		}()
		c.Next()
	}
}

func secretShieldRequestMayHaveBody(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return false
	}
	switch c.Request.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

func secretShieldSkipContentType(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch ct {
	case "multipart/form-data", "application/octet-stream", "application/pdf", "image/png", "image/jpeg", "image/webp", "audio/mpeg", "audio/wav":
		return true
	default:
		return false
	}
}

func resetSecretShieldRequestBody(c *gin.Context, body []byte) {
	if c == nil || c.Request == nil {
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	c.Request.Header.Del("Content-Encoding")
	c.Request.Header.Del("Content-Length")
}

func newSecretShieldVaultForAPIKey(apiKey *service.APIKey) *service.SecretShieldVault {
	if !secretShieldEnabled(apiKey) {
		return nil
	}
	return service.NewSecretShieldVault()
}

type secretShieldResponseWriter struct {
	gin.ResponseWriter
	vault    *service.SecretShieldVault
	restorer *service.SecretShieldStreamRestorer
}

func installSecretShieldResponseWriter(c *gin.Context, vault *service.SecretShieldVault) *secretShieldResponseWriter {
	if c == nil || vault == nil || vault.Empty() {
		return nil
	}
	if existing, ok := c.Get(secretShieldWriterContextKey); ok {
		if writer, ok := existing.(*secretShieldResponseWriter); ok && writer != nil {
			return writer
		}
	}
	writer := &secretShieldResponseWriter{
		ResponseWriter: c.Writer,
		vault:          vault,
		restorer:       vault.NewStreamRestorer(),
	}
	c.Writer = writer
	c.Set(secretShieldWriterContextKey, writer)
	return writer
}

func (w *secretShieldResponseWriter) WriteHeader(code int) {
	w.prepareHeadersForRestore()
	w.ResponseWriter.WriteHeader(code)
}

func (w *secretShieldResponseWriter) WriteHeaderNow() {
	w.prepareHeadersForRestore()
	w.ResponseWriter.WriteHeaderNow()
}

func (w *secretShieldResponseWriter) Write(data []byte) (int, error) {
	if w == nil || w.vault == nil || w.shouldBypassRestore() {
		return w.ResponseWriter.Write(data)
	}
	w.prepareHeadersForRestore()
	w.ResponseWriter.WriteHeaderNow()
	out := w.restorer.RestoreChunk(data, false)
	if len(out) == 0 {
		return len(data), nil
	}
	_, err := w.ResponseWriter.Write(out)
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

func (w *secretShieldResponseWriter) WriteString(s string) (int, error) {
	if w == nil || w.vault == nil || w.shouldBypassRestore() {
		return w.ResponseWriter.WriteString(s)
	}
	w.prepareHeadersForRestore()
	w.ResponseWriter.WriteHeaderNow()
	out := w.restorer.RestoreChunk([]byte(s), false)
	if len(out) == 0 {
		return len(s), nil
	}
	_, err := w.ResponseWriter.Write(out)
	if err != nil {
		return 0, err
	}
	return len(s), nil
}

func (w *secretShieldResponseWriter) Size() int {
	if w == nil || w.ResponseWriter == nil {
		return 0
	}
	size := w.ResponseWriter.Size()
	if w.restorer != nil && w.restorer.BufferedLen() > 0 {
		if size < 0 {
			size = 0
		}
		size += w.restorer.BufferedLen()
	}
	return size
}

func (w *secretShieldResponseWriter) Flush() {
	if w != nil {
		w.prepareHeadersForRestore()
	}
	w.ResponseWriter.Flush()
}

func (w *secretShieldResponseWriter) Finalize() error {
	if w == nil || w.vault == nil || w.restorer == nil || w.shouldBypassRestore() {
		return nil
	}
	out := w.restorer.Finalize()
	if len(out) == 0 {
		return nil
	}
	w.prepareHeadersForRestore()
	w.ResponseWriter.WriteHeaderNow()
	_, err := w.ResponseWriter.Write(out)
	return err
}

func (w *secretShieldResponseWriter) prepareHeadersForRestore() {
	if w == nil || w.ResponseWriter == nil || w.shouldBypassRestore() {
		return
	}
	// Restoring placeholders can change the response byte length. Do not forward
	// an upstream Content-Length once this writer is active.
	w.Header().Del("Content-Length")
}

func (w *secretShieldResponseWriter) shouldBypassRestore() bool {
	if w == nil || w.ResponseWriter == nil {
		return true
	}
	if w.Status() >= http.StatusBadRequest {
		// Avoid restoring secrets into error responses that may be captured by ops
		// error logging. Successful model responses are still restored.
		return true
	}
	encoding := strings.TrimSpace(w.Header().Get("Content-Encoding"))
	return encoding != "" && !strings.EqualFold(encoding, "identity")
}
