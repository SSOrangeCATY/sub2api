//go:build unit

package admin

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type billingIdentityAdminStub struct {
	service.AdminService
	migrations int
	actor      int64
}

func (s *billingIdentityAdminStub) MigratePersonalRatesToPurchase(_ context.Context, actor int64) (*service.BillingIdentityMigrationResult, error) {
	s.migrations++
	s.actor = actor
	return &service.BillingIdentityMigrationResult{BackupID: 7, MigratedUsers: 2, ClearedRates: 5, UserIDs: []int64{1, 2}}, nil
}

func TestBillingIdentityMigrationRequiresFullAdminAndConfirmation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name, body    string
		scoped        bool
		status, calls int
	}{
		{"full admin confirmed", "{\"confirm\":true}", false, 200, 1},
		{"confirmation missing", "{}", false, 400, 0},
		{"false confirmation", "{\"confirm\":false}", false, 400, 0},
		{"scoped key forbidden", "{\"confirm\":true}", true, 403, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := &billingIdentityAdminStub{}
			h := NewUserHandler(svc, nil, nil, nil, nil, nil, nil)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9})
				if tt.scoped {
					c.Request = c.Request.WithContext(adminaccess.WithPrincipal(c.Request.Context(), &adminaccess.Principal{KeyID: "test"}))
				}
				c.Next()
			})
			router.POST("/migrate", h.MigrateBillingIdentity)
			req := httptest.NewRequest("POST", "/migrate", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, tt.status, w.Code, w.Body.String())
			require.Equal(t, tt.calls, svc.migrations)
			if tt.calls > 0 {
				require.Equal(t, int64(9), svc.actor)
				require.Contains(t, w.Body.String(), "\"cleared_rates\":5")
				require.NotContains(t, w.Body.String(), "UserIDs")
			}
		})
	}
}
