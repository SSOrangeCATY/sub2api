package middleware

import (
	"errors"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

const ContextKeyMonitorFallbackEntry = "monitor_fallback_entry"
const ContextKeyMonitorFallbackRoute = "monitor_fallback_route"

var errFallbackRequestBody = errors.New("failed to read monitor fallback request body")

// Entry access has already been authorized; resolve before billing enforcement.
func resolveAuthenticatedMonitorFallback(c *gin.Context, keys *service.APIKeyService, key *service.APIKey) (*service.APIKey, error) {
	if key == nil || key.Group == nil || key.Group.GroupType != service.GroupTypeFallback {
		return key, nil
	}
	if monitorFallbackStatefulPath(c.Request.URL.Path) {
		return nil, infraerrors.BadRequest("FALLBACK_STATEFUL_ENDPOINT_UNSUPPORTED", "use a business-group key for videos, provider tasks, batch images, Live sessions and Gemini cached contents; these resources must remain bound to their original group")
	}
	if c.Request.Method == http.MethodGet && (strings.HasSuffix(c.Request.URL.Path, "/usage") || strings.HasSuffix(c.Request.URL.Path, "/sub2api/billing")) {
		return key, nil
	}
	if c.Request.Method == http.MethodGet && isAsyncImageTaskRead(c.Request.Method, c.Request.URL.Path) {
		return key, nil
	}
	if isResponsesWebSocketRoute(c) {
		// The actual model arrives in the first frame. Resolve there, rather than
		// rejecting a usable model based on an unrelated primary-model probe.
		c.Set(ContextKeyMonitorFallbackEntry, key)
		return key, nil
	}
	model, models, err := monitorFallbackRequestModels(c)
	if err != nil {
		return nil, err
	}
	for _, candidate := range models {
		if candidate != "" && !key.Group.ModelAllowlist.Allows(candidate) {
			return nil, service.ErrMonitorFallbackModelDenied
		}
	}
	routed, route, err := keys.ResolveMonitorFallbackKey(c.Request.Context(), key, model)
	if err != nil {
		return nil, err
	}
	c.Set(ContextKeyMonitorFallbackEntry, key)
	c.Set(ContextKeyMonitorFallbackRoute, route)
	return routed, nil
}

func monitorFallbackStatefulPath(path string) bool {
	for _, segment := range []string{"/videos", "/contents/generations/tasks", "/cachedContents", "/images/batches", "/live", "/realtime/calls"} {
		if i := strings.Index(path, segment); i >= 0 && (i+len(segment) == len(path) || path[i+len(segment)] == '/') {
			return true
		}
	}
	return false
}

func monitorFallbackRequestModels(c *gin.Context) (string, []string, error) {
	model := groupModelAllowlistModelFromParams(c)
	query := strings.TrimSpace(c.Query("model"))
	models := []string{}
	if model != "" {
		models = append(models, model)
	}
	if query != "" {
		models = append(models, query)
	}
	if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch {
		bodyModels, ok := groupModelAllowlistModelsFromBody(c)
		if !ok {
			return "", nil, errFallbackRequestBody
		}
		models = append(models, bodyModels...)
		if model == "" && len(bodyModels) > 0 {
			model = bodyModels[0]
		}
	}
	// Route using the model the gateway executes: provider path, then body.
	// Query models remain checked by the allowlist, but cannot override a body.
	if model == "" {
		model = query
	}
	return model, models, nil
}
