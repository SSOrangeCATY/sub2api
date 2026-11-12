//go:build unit

package middleware

import (
	"context"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type qqGateUserRepo struct {
	service.UserRepository
	err   error
	calls []int64
}

func (r *qqGateUserRepo) BeginUserRiskRequest(_ context.Context, userID int64, policy, scope string) (service.UserRiskAdmission, error) {
	return service.UserRiskAdmission{UserID: userID, PolicyKey: policy, Scope: scope,
		EventID: rand.Text(), Generation: 1, AdmittedAt: time.Now()}, nil
}

func (r *qqGateUserRepo) CheckQQAPIAccess(_ context.Context, uid int64) error {
	r.calls = append(r.calls, uid)
	return r.err
}

func TestQQAPIAccessBothProtocolsAndSimpleMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, google := range []bool{false, true} {
		for _, mode := range []string{config.RunModeStandard, config.RunModeSimple} {
			t.Run(mode+map[bool]string{true: "_google", false: "_standard"}[google], func(t *testing.T) {
				cfg := &config.Config{RunMode: mode}
				key := &service.APIKey{ID: 1, Key: "qq-gate-test-key", Status: service.StatusActive, UserID: 7, User: &service.User{ID: 7, Status: service.StatusActive, Balance: 10}}
				keyRepo := &stubApiKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) { return key, nil }}
				userRepo := &qqGateUserRepo{}
				svc := service.NewAPIKeyService(keyRepo, userRepo, nil, nil, nil, nil, cfg)
				r := gin.New()
				if google {
					r.Use(APIKeyAuthWithSubscriptionGoogle(svc, nil, cfg))
				} else {
					r.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg)))
				}
				r.GET("/api", func(c *gin.Context) { c.Status(200) })
				// Change the persisted gate between requests made with the same key/cache.
				for _, tc := range []struct {
					err    error
					status int
				}{
					{nil, 200},
					{infraerrors.Forbidden("QQ_VERIFICATION_REQUIRED", "请先完成 QQ 认证"), 403},
					{infraerrors.ServiceUnavailable("QQ_VERIFICATION_UNAVAILABLE", "QQ verification unavailable"), 503},
					{nil, 200},
				} {
					userRepo.err = tc.err
					w := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodGet, "/api", nil)
					req.Header.Set("Authorization", "Bearer "+key.Key)
					r.ServeHTTP(w, req)
					require.Equal(t, tc.status, w.Code, w.Body.String())
				}
				require.Equal(t, []int64{7, 7, 7, 7}, userRepo.calls)
			})
		}
	}
}
