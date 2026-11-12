package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const nonStreamMeteringJSON = `{"id":"resp_metering_content_type","object":"response","model":"gpt-6-astra","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"OK; literal data: example"}]}],"usage":{"input_tokens":4125,"output_tokens":5,"total_tokens":4130,"input_tokens_details":{"cached_tokens":3373}}}`

func forwardNonStreamMeteringFixture(t *testing.T, passthrough bool, header, body string, filteredHeaders bool) (*httptest.ResponseRecorder, *OpenAIForwardResult, *Account) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(""))
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{header}, "X-Request-Id": []string{"metering-content-type"}}, Body: io.NopCloser(strings.NewReader(body))}}
	cfg := &config.Config{}
	cfg.Security.ResponseHeaders.Enabled = filteredHeaders
	cfg.Security.ResponseHeaders.AdditionalAllowed = []string{"Content-Type"}
	service := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream, responseHeaderFilter: compileResponseHeaderFilter(cfg)}
	account := &Account{ID: 17, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "synthetic-test-key"}, Extra: map[string]any{"openai_passthrough": passthrough}}
	result, err := service.Forward(context.Background(), c, account, []byte(`{"model":"gpt-6-astra","instructions":"Reply OK.","input":"Reply OK.","stream":false}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	return recorder, result, account
}

func TestOpenAINonStreamMislabeledJSONRetainsReportedUsage(t *testing.T) {
	for _, filteredHeaders := range []bool{false, true} {
		for _, passthrough := range []bool{false, true} {
			name := "standard"
			if passthrough {
				name = "passthrough"
			}
			if filteredHeaders {
				name += "/filtered_headers"
			}
			t.Run(name, func(t *testing.T) {
				recorder, result, account := forwardNonStreamMeteringFixture(t, passthrough, "text/event-stream; charset=utf-8", nonStreamMeteringJSON, filteredHeaders)
				require.Equal(t, 4125, result.Usage.InputTokens)
				require.Equal(t, 5, result.Usage.OutputTokens)
				require.Equal(t, 3373, result.Usage.CacheReadInputTokens)
				require.True(t, strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json"), recorder.Header().Get("Content-Type"))
				require.True(t, gjson.ValidBytes(recorder.Body.Bytes()))
				require.Equal(t, int64(4125), gjson.GetBytes(recorder.Body.Bytes(), "usage.input_tokens").Int())

				logs := &openAIRecordUsageLogRepoStub{inserted: true}
				billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
				billing := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
				key := openAIRecordUsageAPIKeyWithGroup(billing, 4, false)
				key.GroupID = &key.Group.ID
				key.Group.RateMultiplier = 1
				input, output, cache := 1e-6, 2e-6, .1e-6
				key.Group.ModelPricing = []ChannelModelPricing{{Models: []string{"gpt-6-astra"}, BillingMode: BillingModeToken, InputPrice: &input, OutputPrice: &output, CacheReadPrice: &cache}}
				err := billing.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: result, APIKey: key, User: &User{ID: 7}, Account: account})
				require.NoError(t, err)
				require.NotNil(t, logs.lastLog)
				require.Equal(t, 752, logs.lastLog.InputTokens)
				require.Equal(t, 3373, logs.lastLog.CacheReadTokens)
				require.Equal(t, 5, logs.lastLog.OutputTokens)
				require.InDelta(t, .0010993, logs.lastLog.TotalCost, 1e-12)
				require.InDelta(t, .0010993, logs.lastLog.ActualCost, 1e-12)
			})
		}
	}
}

func TestOpenAINonStreamUsageContentTypeControls(t *testing.T) {
	sse := "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":" + nonStreamMeteringJSON + "}\n\n"
	for _, passthrough := range []bool{false, true} {
		for _, test := range []struct{ name, header, body string }{{"json", "application/json", nonStreamMeteringJSON}, {"framed_sse", "text/event-stream", sse}} {
			name := test.name + "/standard"
			if passthrough {
				name = test.name + "/passthrough"
			}
			t.Run(name, func(t *testing.T) {
				recorder, result, _ := forwardNonStreamMeteringFixture(t, passthrough, test.header, test.body, false)
				require.Equal(t, 4125, result.Usage.InputTokens)
				require.Equal(t, 5, result.Usage.OutputTokens)
				require.Equal(t, 3373, result.Usage.CacheReadInputTokens)
				require.True(t, gjson.ValidBytes(recorder.Body.Bytes()))
				require.True(t, strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json"))
			})
		}
	}
}
