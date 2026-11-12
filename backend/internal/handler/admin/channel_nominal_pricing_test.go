//go:build unit

package admin

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNominalPriceAPISelectsOnlyExplicitNativeCard(t *testing.T) {
	h := &ChannelHandler{}
	for _, tc := range []struct {
		query  string
		status int
		found  bool
	}{
		{"model=deepseek-flash&source_currency=CNY", 200, true},
		{"model=unknown&source_currency=CNY", 200, false},
		{"model=deepseek-flash&source_currency=EUR", 200, false},
		{"model=deepseek-flash&source_currency=cny", 400, false},
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/?"+tc.query, nil)
		h.GetModelDefaultPricing(c)
		require.Equal(t, tc.status, w.Code)
		if w.Code == 200 {
			var body struct {
				Data struct {
					Found              bool    `json:"found"`
					SourceCurrency     string  `json:"source_currency"`
					SettlementCurrency string  `json:"settlement_currency"`
					InputPrice         float64 `json:"input_price"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Equal(t, tc.found, body.Data.Found)
			require.Equal(t, "USD", body.Data.SettlementCurrency)
			if tc.found {
				require.Equal(t, 1e-6, body.Data.InputPrice)
			}
		}
	}
}

func TestNominalPriceCurrencyRequestResponseRoundTrip(t *testing.T) {
	r := channelModelPricingRequest{SourceCurrency: "CNY", Models: []string{"any-model"}}
	price := pricingRequestToService([]channelModelPricingRequest{r}, true)[0]
	require.Equal(t, "CNY", price.SourceCurrency)
	require.Equal(t, "CNY", pricingToResponse(&price).SourceCurrency)
}
