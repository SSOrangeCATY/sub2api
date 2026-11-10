package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrWelfareUnverified     = infraerrors.Forbidden("WELFARE_UNVERIFIED", "请先在网站个人资料中生成认证码，再在指定 QQ 群 @机器人 发送：认证 认证码")
	ErrWelfareGroup          = infraerrors.Forbidden("WELFARE_GROUP_REQUIRED", "福利中心仅限已配置的 QQ 群，请在指定群内 @机器人 使用。")
	ErrWelfareUnavailable    = infraerrors.Conflict("WELFARE_UNAVAILABLE", "福利中心未启用或认证群尚未配置。")
	ErrWelfareAlreadyClaimed = infraerrors.Conflict("WELFARE_ALREADY_CLAIMED", "该网站账号已领取过此兑换码。")
	ErrWelfareSoldOut        = infraerrors.Conflict("WELFARE_SOLD_OUT", "商品或兑换码已下架、到期或领完。")
	ErrWelfarePoints         = infraerrors.Conflict("WELFARE_POINTS_REQUIRED", "积分不足，请先签到。")
	ErrWelfareBinding        = infraerrors.Conflict("WELFARE_BINDING_INVALID", "认证码无效、已过期，或 QQ / 网站账号已绑定。请重新生成认证码；更换账号请联系管理员。")
	ErrWelfareRate           = infraerrors.TooManyRequests("WELFARE_RATE_LIMITED", "操作过于频繁，请稍后再试。")
)

type QQWelfareConfig struct {
	RequireQQForAPI bool       `json:"require_qq_for_api"`
	Enabled         bool       `json:"enabled"`
	GroupNumber     string     `json:"group_number"`
	GroupOpenID     string     `json:"group_open_id"`
	LastBotSeenAt   *time.Time `json:"last_bot_seen_at"`
	BotKey          string     `json:"-"`
}
type QQWelfareProduct struct {
	ID           int64      `json:"id"`
	Version      int64      `json:"version"`
	Title        string     `json:"title"`
	PointsCost   int        `json:"points_cost"`
	BalanceValue float64    `json:"balance_value"`
	Stock        int        `json:"stock"`
	Enabled      bool       `json:"enabled"`
	ExpiresAt    *time.Time `json:"expires_at"`
}
type QQWelfareCoupon struct {
	MinPaidBalance float64    `json:"min_paid_balance"`
	ID             int64      `json:"id"`
	Code           string     `json:"code"`
	Title          string     `json:"title"`
	BalanceValue   float64    `json:"balance_value"`
	MaxClaims      int        `json:"max_claims"`
	Claimed        int        `json:"claimed"`
	Enabled        bool       `json:"enabled"`
	ExpiresAt      *time.Time `json:"expires_at"`
}
type QQWelfareBinding struct {
	RequireQQForAPI bool   `json:"require_qq_for_api"`
	Verified        bool   `json:"verified"`
	GroupNumber     string `json:"group_number"`
	Ready           bool   `json:"ready"`
}
type QQWelfareOrder struct {
	ID           string    `json:"id"`
	UserID       int64     `json:"user_id"`
	Title        string    `json:"title"`
	PointsCost   int       `json:"points_cost"`
	BalanceValue float64   `json:"balance_value"`
	CreatedAt    time.Time `json:"created_at"`
}
type QQWelfareSnapshot struct {
	PaidBalanceTotal float64            `json:"paid_balance_total"`
	Points           int64              `json:"points"`
	CheckedIn        bool               `json:"checked_in"`
	GroupNumber      string             `json:"group_number"`
	Products         []QQWelfareProduct `json:"products"`
	Orders           []QQWelfareOrder   `json:"orders"`
	GeneratedAt      time.Time          `json:"generated_at"`
}
type QQWelfareAdminOrder struct {
	QQWelfareOrder
	Username   string `json:"username"`
	Email      string `json:"email"`
	CouponID   *int64 `json:"coupon_id"`
	CouponCode string `json:"coupon_code"`
}
type QQWelfareCouponClaims struct {
	Orders []QQWelfareAdminOrder `json:"orders"`
	Total  int64                 `json:"total"`
}
type QQWelfareAdmin struct {
	Config           *QQWelfareConfig      `json:"config"`
	Products         []QQWelfareProduct    `json:"products"`
	Coupons          []QQWelfareCoupon     `json:"coupons"`
	Orders           []QQWelfareAdminOrder `json:"orders"`
	VerifiedAccounts int64                 `json:"verified_accounts"`
}
type QQWelfareEvent struct {
	IsWebsite      bool   `json:"-"`
	ProductVersion int64  `json:"-"`
	Action         string `json:"action"`
	GroupID        string `json:"group_id"`
	ActorID        string `json:"actor_id"`
	MessageID      string `json:"message_id"`
	Code           string `json:"code"`
	ProductID      int64  `json:"product_id"`
}
type QQWelfareResult struct {
	Message  string             `json:"message"`
	Snapshot *QQWelfareSnapshot `json:"snapshot,omitempty"`
	Points   *QQWelfarePoints   `json:"points,omitempty"`
}
type QQWelfarePoints struct {
	Points      int64     `json:"points"`
	CheckedIn   bool      `json:"checked_in"`
	GroupNumber string    `json:"group_number"`
	GeneratedAt time.Time `json:"generated_at"`
}
type QQWelfareRepository interface {
	UserIdentity(context.Context, int64) (*QQWelfareEvent, error)
	Config(context.Context) (*QQWelfareConfig, error)
	UpdateConfig(context.Context, bool, string, ...bool) error
	CouponClaims(context.Context, int64, int, int) (*QQWelfareCouponClaims, error)
	GroupCode(context.Context, string, time.Time) error
	BindGroup(context.Context, string, string) error
	Binding(context.Context, int64) (*QQWelfareBinding, error)
	CreateBinding(context.Context, int64, string, time.Time) error
	Verify(context.Context, QQWelfareEvent, string) error
	Snapshot(context.Context, QQWelfareEvent) (*QQWelfareSnapshot, error)
	CheckIn(context.Context, QQWelfareEvent) (bool, int64, error)
	Fulfill(context.Context, QQWelfareEvent, string, string) (*QQWelfareOrder, error)
	Admin(context.Context) (*QQWelfareAdmin, error)
	SaveProduct(context.Context, *QQWelfareProduct) error
	SaveCoupon(context.Context, *QQWelfareCoupon) error
	TouchAndLimit(context.Context, QQWelfareEvent) error
}

type QQWelfareService struct {
	repo    QQWelfareRepository
	billing *BillingCacheService
	auth    APIKeyAuthCacheInvalidator
}

func NewQQWelfareService(repo QQWelfareRepository, billing *BillingCacheService, auth APIKeyAuthCacheInvalidator) *QQWelfareService {
	return &QQWelfareService{repo: repo, billing: billing, auth: auth}
}
func welfareToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(b)), nil
}
func WelfareHash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func (s *QQWelfareService) Authenticate(ctx context.Context, key string) error {
	cfg, err := s.repo.Config(ctx)
	if err != nil {
		return err
	}
	expected, got := sha256.Sum256([]byte(cfg.BotKey)), sha256.Sum256([]byte(key))
	if key == "" || subtle.ConstantTimeCompare(expected[:], got[:]) != 1 {
		return infraerrors.Unauthorized("WELFARE_BOT_AUTH_REQUIRED", "机器人认证失败。")
	}
	return nil
}
func (s *QQWelfareService) Binding(ctx context.Context, uid int64) (*QQWelfareBinding, error) {
	return s.repo.Binding(ctx, uid)
}
func (s *QQWelfareService) BindingCode(ctx context.Context, uid int64) (string, error) {
	code, err := welfareToken(16)
	if err != nil {
		return "", err
	}
	err = s.repo.CreateBinding(ctx, uid, WelfareHash(code), time.Now().Add(10*time.Minute))
	return code, err
}
func (s *QQWelfareService) Admin(ctx context.Context) (*QQWelfareAdmin, error) {
	return s.repo.Admin(ctx)
}

func (s *QQWelfareService) CouponClaims(ctx context.Context, id int64, page, size int) (*QQWelfareCouponClaims, error) {
	if id <= 0 || page < 1 || page > 1000000 || size < 1 || size > 100 {
		return nil, infraerrors.BadRequest("WELFARE_INVALID_PAGE", "无效的详情分页。")
	}
	return s.repo.CouponClaims(ctx, id, page, size)
}

var welfareGroupPattern = regexp.MustCompile(`^[0-9]{5,16}$`)
var welfareCouponPattern = regexp.MustCompile(`^[A-Z0-9_-]{3,32}$`)

func (s *QQWelfareService) UpdateConfig(ctx context.Context, enabled bool, group string, requireQQ ...bool) error {
	group = strings.TrimSpace(group)
	if !welfareGroupPattern.MatchString(group) {
		return infraerrors.BadRequest("WELFARE_INVALID_GROUP", "请输入有效的 QQ 群号。")
	}
	return s.repo.UpdateConfig(ctx, enabled, group, requireQQ...)
}
func (s *QQWelfareService) GroupCode(ctx context.Context) (string, error) {
	code, err := welfareToken(16)
	if err != nil {
		return "", err
	}
	err = s.repo.GroupCode(ctx, WelfareHash(code), time.Now().Add(10*time.Minute))
	return code, err
}
func validateWelfareValue(title string, value float64, expiry *time.Time, enabled bool) error {
	decimal := strings.SplitN(strconv.FormatFloat(value, 'f', -1, 64), ".", 2)
	if strings.TrimSpace(title) == "" || len([]rune(title)) > 80 || math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 || value > 10000 || (len(decimal) == 2 && len(decimal[1]) > 8) {
		return infraerrors.BadRequest("WELFARE_INVALID_PRODUCT", "名称限 1–80 字，余额须大于 0 且不超过 10000，最多 8 位小数。")
	}
	if enabled && expiry != nil && !expiry.After(time.Now()) {
		return infraerrors.BadRequest("WELFARE_INVALID_EXPIRY", "截止时间必须晚于当前时间。")
	}
	return nil
}
func (s *QQWelfareService) SaveProduct(ctx context.Context, p *QQWelfareProduct) error {
	if err := validateWelfareValue(p.Title, p.BalanceValue, p.ExpiresAt, p.Enabled); err != nil {
		return err
	}
	p.Title = strings.TrimSpace(p.Title)
	if p.ID < 0 || p.PointsCost < 1 || p.PointsCost > 1000000 || p.Stock < 0 || p.Stock > 1000000 {
		return infraerrors.BadRequest("WELFARE_INVALID_PRODUCT", "积分价格须为 1–1000000，库存须为 0–1000000。")
	}
	return s.repo.SaveProduct(ctx, p)
}
func (s *QQWelfareService) SaveCoupon(ctx context.Context, p *QQWelfareCoupon) error {
	p.Code = strings.ToUpper(strings.TrimSpace(p.Code))
	p.Title = strings.TrimSpace(p.Title)
	if err := validateWelfareValue(p.Title, p.BalanceValue, p.ExpiresAt, p.Enabled); err != nil {
		return err
	}
	paidDecimal := strings.SplitN(strconv.FormatFloat(p.MinPaidBalance, 'f', -1, 64), ".", 2)
	if (len(paidDecimal) == 2 && len(paidDecimal[1]) > 8) || math.IsNaN(p.MinPaidBalance) || math.IsInf(p.MinPaidBalance, 0) || p.MinPaidBalance < 0 || p.MinPaidBalance > 100000000 {
		return infraerrors.BadRequest("WELFARE_INVALID_PAID_THRESHOLD", "付费门槛须为 0–100000000。")
	}
	if p.ID < 0 || !welfareCouponPattern.MatchString(p.Code) || p.MaxClaims < 0 || p.MaxClaims > 1000000 {
		return infraerrors.BadRequest("WELFARE_INVALID_COUPON", "兑换码限 3–32 位字母、数字、下划线或短横线，领取上限为 0–1000000（0 为不限）。")
	}
	return s.repo.SaveCoupon(ctx, p)
}

// UserAction derives the QQ identity from the authenticated website user, never the request body.
func (s *QQWelfareService) UserAction(ctx context.Context, uid int64, action, requestID, code string, productID, productVersion int64) (*QQWelfareResult, error) {
	if action != "overview" && action != "checkin" && action != "exchange" && action != "claim" {
		return nil, infraerrors.BadRequest("WELFARE_INVALID_ACTION", "未知福利操作。")
	}
	if (action == "exchange" || action == "claim") && !regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`).MatchString(requestID) {
		return nil, infraerrors.BadRequest("WELFARE_INVALID_REQUEST", "缺少有效的请求编号。")
	}
	e, err := s.repo.UserIdentity(ctx, uid)
	if err != nil {
		return nil, err
	}
	e.IsWebsite, e.Action, e.MessageID, e.Code, e.ProductID, e.ProductVersion = true, action, "website:"+requestID, code, productID, productVersion
	return s.event(ctx, *e)
}
func (s *QQWelfareService) Event(ctx context.Context, e QQWelfareEvent) (*QQWelfareResult, error) {
	// Even in-process callers cannot opt the bot endpoint into website permissions.
	e.IsWebsite = false
	if e.Action != "points" && e.Action != "checkin" && e.Action != "verify" && e.Action != "bind-group" {
		return nil, infraerrors.BadRequest("WELFARE_INVALID_ACTION", "机器人仅支持签到、积分查询和群认证，兑换请前往网站福利中心。")
	}
	return s.event(ctx, e)
}
func (s *QQWelfareService) event(ctx context.Context, e QQWelfareEvent) (*QQWelfareResult, error) {
	if len(e.GroupID) < 1 || len(e.GroupID) > 128 || len(e.ActorID) < 1 || len(e.ActorID) > 128 || len(e.MessageID) < 1 || len(e.MessageID) > 256 || len(e.Code) > 64 {
		return nil, ErrWelfareGroup
	}
	if err := s.repo.TouchAndLimit(ctx, e); err != nil {
		return nil, err
	}
	e.Code = strings.ToUpper(strings.TrimSpace(e.Code))
	switch e.Action {
	case "bind-group":
		if err := s.repo.BindGroup(ctx, WelfareHash(e.Code), e.GroupID); err != nil {
			return nil, err
		}
		return &QQWelfareResult{Message: "认证群连接成功。群成员现在可以发送：认证 认证码"}, nil
	case "verify":
		if err := s.repo.Verify(ctx, e, WelfareHash(e.Code)); err != nil {
			return nil, err
		}
		return &QQWelfareResult{Message: "Q 群认证成功，长期有效。可在网站福利中心签到和兑换，群内发送「签到」或「积分」。"}, nil
	case "overview", "points":
		snap, err := s.repo.Snapshot(ctx, e)
		if err != nil {
			return nil, err
		}
		if e.Action == "points" {
			return &QQWelfareResult{Points: &QQWelfarePoints{Points: snap.Points, CheckedIn: snap.CheckedIn, GroupNumber: snap.GroupNumber, GeneratedAt: snap.GeneratedAt}}, nil
		}
		return &QQWelfareResult{Snapshot: snap}, nil
	case "checkin":
		fresh, points, err := s.repo.CheckIn(ctx, e)
		if err != nil {
			return nil, err
		}
		message := "今天已签到"
		if fresh {
			message = "签到成功，获得 1 积分"
		}
		return &QQWelfareResult{Message: fmt.Sprintf("%s。当前积分：%d（北京时间每日 00:00 更新）。", message, points)}, nil
	case "exchange", "claim":
		if e.Action == "exchange" && e.ProductID <= 0 {
			return nil, infraerrors.BadRequest("WELFARE_INVALID_PRODUCT", "请选择有效的积分商品。")
		}
		if e.Action == "claim" && !welfareCouponPattern.MatchString(e.Code) {
			return nil, ErrWelfareSoldOut
		}
		id, err := welfareToken(16)
		if err != nil {
			return nil, err
		}
		code, err := welfareToken(16)
		if err != nil {
			return nil, err
		}
		order, err := s.repo.Fulfill(ctx, e, id, code)
		if err != nil {
			return nil, err
		}
		if s.auth != nil {
			s.auth.InvalidateAuthCacheByUserID(ctx, order.UserID)
		}
		if s.billing != nil {
			cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_ = s.billing.InvalidateUserBalance(cacheCtx, order.UserID)
		}
		return &QQWelfareResult{Message: fmt.Sprintf("领取成功：%s\n兑换码已自动核销，$%s 已存入绑定的网站账号。\n消耗 %d 积分 / 订单 %s", order.Title, strconv.FormatFloat(order.BalanceValue, 'f', -1, 64), order.PointsCost, order.ID)}, nil
	default:
		return nil, infraerrors.BadRequest("WELFARE_INVALID_ACTION", "未知福利命令。")
	}
}
