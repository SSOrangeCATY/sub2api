package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMonitorFallbackRequestModelsPreservesPathAndBody(t *testing.T) {
	for _, tc := range []struct {
		name, path, url, body, model string
		want                         []string
	}{
		{"gemini path", "/v1beta/models/*modelAction", "/v1beta/models/blocked:generateContent", `{"contents":[]}`, "blocked", []string{"blocked"}},
		{"path plus body", "/v1beta/models/*modelAction", "/v1beta/models/path-model:generateContent?model=query-model", `{"model":"body-model"}`, "path-model", []string{"path-model", "query-model", "body-model"}},
		{"responses body", "/v1/responses", "/v1/responses", `{"model":"public-alias","input":"hello"}`, "public-alias", []string{"public-alias"}},
		{"body overrides query for routing", "/v1/responses", "/v1/responses?model=unrelated-healthy-model", `{"model":"actual-model","input":"hello"}`, "actual-model", []string{"unrelated-healthy-model", "actual-model"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.POST(tc.path, func(c *gin.Context) {
				model, candidates, err := monitorFallbackRequestModels(c)
				require.NoError(t, err)
				require.Equal(t, tc.model, model)
				require.Equal(t, tc.want, candidates)
				body, err := io.ReadAll(c.Request.Body)
				require.NoError(t, err)
				require.Equal(t, tc.body, string(body))
				c.Status(204)
			})
			response := doJSON(t, router, http.MethodPost, tc.url, tc.body)
			require.Equal(t, 204, response.Code)
		})
	}
}

func TestMonitorFallbackEntryRejectsGeminiPathBeforeResolving(t *testing.T) {
	router := gin.New()
	key := allowlistAPIKey(true, "allowed")
	key.Group.GroupType = service.GroupTypeFallback
	router.POST("/v1beta/models/*modelAction", func(c *gin.Context) {
		_, err := resolveAuthenticatedMonitorFallback(c, nil, key)
		require.ErrorIs(t, err, service.ErrMonitorFallbackModelDenied)
		c.Status(404)
	})
	require.Equal(t, 404, doJSON(t, router, http.MethodPost, "/v1beta/models/blocked:generateContent", `{"contents":[]}`).Code)
}

func TestMonitorFallbackWSDefersRoutingUntilFirstModel(t *testing.T) {
	key := &service.APIKey{Group: &service.Group{GroupType: service.GroupTypeFallback}}
	router := gin.New()
	router.GET("/v1/responses", func(c *gin.Context) {
		got, err := resolveAuthenticatedMonitorFallback(c, nil, key)
		require.NoError(t, err)
		require.Same(t, key, got)
		entry, ok := c.Get(ContextKeyMonitorFallbackEntry)
		require.True(t, ok)
		require.Same(t, key, entry)
		c.Status(204)
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, 204, response.Code)
}

func TestMonitorFallbackStatefulEndpointsRejectedBeforeUpstream(t *testing.T) {
	for _, path := range []string{"/videos", "/v1/videos/generations/id/content", "/v3/contents/generations/tasks", "/v1beta/cachedContents/cache-id", "/v1/images/batches/id/cancel"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		_, err := resolveAuthenticatedMonitorFallback(c, nil, &service.APIKey{Group: &service.Group{GroupType: service.GroupTypeFallback}})
		require.ErrorContains(t, err, "business-group key")
	}
	require.False(t, monitorFallbackStatefulPath("/v1/images/tasks/id"))
	require.False(t, monitorFallbackStatefulPath("/v1/responses"))
}
