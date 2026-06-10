package admin

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"

	"github.com/gin-gonic/gin"
)

var tinyAnnouncementPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

func TestAnnouncementImageUploadAndServe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewAnnouncementHandler(nil, &config.Config{Pricing: config.PricingConfig{DataDir: t.TempDir()}})
	router := gin.New()
	router.POST("/admin/announcement-images", h.UploadImage)
	router.GET("/announcement-images/:filename", h.ServeImage)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("image", "hello.png")
	require.NoError(t, err)
	_, err = part.Write(tinyAnnouncementPNG)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/admin/announcement-images", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope struct {
		Code int                             `json:"code"`
		Data AnnouncementImageUploadResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, 0, envelope.Code)
	require.Regexp(t, `^/api/v1/announcement-images/[a-f0-9]{32}\.png$`, envelope.Data.URL)
	require.Contains(t, envelope.Data.Markdown, envelope.Data.URL)

	servePath := strings.TrimPrefix(envelope.Data.URL, "/api/v1")
	serveReq := httptest.NewRequest(http.MethodGet, servePath, nil)
	serveRec := httptest.NewRecorder()
	router.ServeHTTP(serveRec, serveReq)
	require.Equal(t, http.StatusOK, serveRec.Code)
	require.Equal(t, tinyAnnouncementPNG, serveRec.Body.Bytes())
}

func TestAnnouncementImageUploadRejectsNonImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewAnnouncementHandler(nil, &config.Config{Pricing: config.PricingConfig{DataDir: t.TempDir()}})
	router := gin.New()
	router.POST("/admin/announcement-images", h.UploadImage)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("image", "note.txt")
	require.NoError(t, err)
	_, err = part.Write([]byte("not an image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/admin/announcement-images", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
