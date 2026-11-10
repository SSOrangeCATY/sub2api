package service

import (
	"crypto/rand"
	"encoding/hex"
	"math"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	BalanceSourceUnknown = "unknown"
	BalanceSourceGift    = "gift"
	BalanceSourcePaid    = "paid"
)

// NormalizeBalanceSource accepts payment provenance only from trusted admin writes.
func NormalizeBalanceSource(source, codeType string, value float64) (string, error) {
	if source == "" {
		source = BalanceSourceGift
	}
	if source != BalanceSourceGift && source != BalanceSourcePaid && source != BalanceSourceUnknown {
		return "", infraerrors.BadRequest("BALANCE_SOURCE_INVALID", "余额来源须为付费或赠送。")
	}
	if source == BalanceSourcePaid && ((codeType != RedeemTypeBalance && codeType != AdjustmentTypeAdminBalance) || value <= 0 || math.IsNaN(value) || math.IsInf(value, 0)) {
		return "", infraerrors.BadRequest("PAID_BALANCE_REQUIRED", "付费来源仅适用于正数余额充值。")
	}
	return source, nil
}

type RedeemCode struct {
	BalanceSource string
	ID            int64
	Code          string
	Type          string
	Value         float64
	Status        string
	UsedBy        *int64
	UsedAt        *time.Time
	Notes         string
	CreatedAt     time.Time
	ExpiresAt     *time.Time

	GroupID      *int64
	ValidityDays int

	User  *User
	Group *Group
}

func (r *RedeemCode) IsUsed() bool {
	return r.Status == StatusUsed
}

func (r *RedeemCode) IsExpired() bool {
	return r.IsExpiredAt(time.Now())
}

func (r *RedeemCode) IsExpiredAt(now time.Time) bool {
	if r == nil {
		return false
	}
	if r.Status == StatusExpired {
		return true
	}
	return r.Status == StatusUnused && r.ExpiresAt != nil && !r.ExpiresAt.After(now)
}

func (r *RedeemCode) CanUse() bool {
	return r.Status == StatusUnused && !r.IsExpired()
}

func GenerateRedeemCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
