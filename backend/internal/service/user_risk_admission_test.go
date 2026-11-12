//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type riskAdmissionCapabilityStub struct {
	UserRepository
	value         UserRiskAdmission
	err           error
	userID        int64
	policy, scope string
}

func (r *riskAdmissionCapabilityStub) BeginUserRiskRequest(_ context.Context, userID int64, policy, scope string) (UserRiskAdmission, error) {
	r.userID, r.policy, r.scope = userID, policy, scope
	return r.value, r.err
}

type riskAdmissionMissingCapability struct{ UserRepository }

func TestUserRiskAdmissionServiceBindsIdentityPolicyAndScope(t *testing.T) {
	repo := &riskAdmissionCapabilityStub{value: UserRiskAdmission{
		UserID: 17, PolicyKey: UserRiskPolicyCyber, Scope: UserRiskScopeAPI,
		EventID: "server-event", Generation: 2, AdmittedAt: time.Unix(10, 0),
	}}
	svc := &APIKeyService{userRepo: repo}
	admission, err := svc.BeginUserRiskRequest(context.Background(), 17)
	if err != nil {
		t.Fatal(err)
	}
	if admission != repo.value || repo.userID != 17 || repo.policy != UserRiskPolicyCyber || repo.scope != UserRiskScopeAPI {
		t.Fatal("request admission was not bound to the authenticated user and server policy")
	}
	repo.value.UserID = 18
	if _, err := svc.BeginUserRiskRequest(context.Background(), 17); err == nil {
		t.Fatal("a mismatched admission identity was accepted")
	}
}

func TestUserRiskAdmissionServiceFailsClosedWithoutCapability(t *testing.T) {
	cases := []*APIKeyService{nil, {}, {userRepo: &riskAdmissionMissingCapability{}}}
	for _, svc := range cases {
		_, err := svc.BeginUserRiskRequest(context.Background(), 17)
		if err == nil || infraerrors.Code(err) != 500 {
			t.Fatal("missing enforcement capability did not reject the request")
		}
	}
	denied := infraerrors.Forbidden("USER_ACCESS_RESTRICTED", "restricted")
	svc := &APIKeyService{userRepo: &riskAdmissionCapabilityStub{err: denied}}
	_, err := svc.BeginUserRiskRequest(context.Background(), 17)
	if err != denied {
		t.Fatal("the authoritative access denial was not preserved")
	}
}
