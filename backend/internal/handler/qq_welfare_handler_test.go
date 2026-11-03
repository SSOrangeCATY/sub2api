package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type welfareHandlerRepo struct{ service.QQWelfareRepository }

func (*welfareHandlerRepo) Config(context.Context) (*service.QQWelfareConfig, error) {
	return &service.QQWelfareConfig{BotKey: "test-bot-key"}, nil
}
func TestQQWelfareHTTPBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewQQWelfareHandler(service.NewQQWelfareService(&welfareHandlerRepo{}, nil, nil))
	for _, tc := range []struct {
		name, key, body string
		want            int
	}{
		{"missing bot key", "", `{}`, http.StatusUnauthorized},
		{"wrong bot key", "wrong", `{}`, http.StatusUnauthorized},
		{"oversized event", "test-bot-key", `{"code":"` + strings.Repeat("a", 5000) + `"}`, http.StatusBadRequest},
		{"bot cannot claim via forged website flag", "test-bot-key", `{"action":"claim","is_website":true,"code":"WELCOME"}`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.POST("/bot", h.Bot)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/bot", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Welfare-Bot-Key", tc.key)
			r.ServeHTTP(w, req)
			require.Equal(t, tc.want, w.Code)
			require.NotContains(t, w.Body.String(), "test-bot-key")
		})
	}
	for _, handler := range []gin.HandlerFunc{h.Binding, h.BindingCode, h.Center, h.UserAction} {
		r := gin.New()
		r.GET("/user", handler)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/user", nil))
		require.Equal(t, http.StatusUnauthorized, w.Code)
	}
	for _, handler := range []gin.HandlerFunc{h.Admin, h.Config, h.GroupCode, h.Product, h.Coupon} {
		r := gin.New()
		r.POST("/admin", handler)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/admin", nil)
		req = req.WithContext(adminaccess.WithPrincipal(req.Context(), &adminaccess.Principal{KeyID: "scoped"}))
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusForbidden, w.Code)
	}
}
