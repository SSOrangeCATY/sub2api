package service

import (
	_ "embed"
	"encoding/json"
	"strings"
)

// Source currency is provenance, not a request-time FX policy. Values have
// already been re-denominated as USD numbers when the operator saves the card.
// Empty provenance preserves legacy cards without claiming their original unit.
func IsValidPricingSourceCurrency(currency string) bool {
	if currency == "" {
		return true
	}
	if len(currency) != 3 {
		return false
	}
	for _, c := range currency {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

//go:embed nominal_price_presets.json
var nominalPricePresetJSON []byte

type NominalPricePreset struct {
	Found              bool    `json:"found"`
	SourceCurrency     string  `json:"source_currency"`
	SettlementCurrency string  `json:"settlement_currency"`
	UnitPolicy         string  `json:"unit_policy"`
	VerifiedAt         string  `json:"verified_at"`
	InputPrice         float64 `json:"input_price"`
	OutputPrice        float64 `json:"output_price"`
	CacheReadPrice     float64 `json:"cache_read_price"`
	CacheWritePrice    float64 `json:"cache_write_price"`
}

var nominalPricePresets = func() map[string]map[string]NominalPricePreset {
	var presets map[string]map[string]NominalPricePreset
	if err := json.Unmarshal(nominalPricePresetJSON, &presets); err != nil {
		panic(err)
	}
	return presets
}()

// LookupNominalPricePreset selects an explicitly recorded native price card.
// It never derives one denomination from another or guesses unknown aliases.
func LookupNominalPricePreset(model, currency string) (NominalPricePreset, bool) {
	preset, ok := nominalPricePresets[strings.ToLower(strings.TrimSpace(model))][currency]
	if !ok {
		return NominalPricePreset{}, false
	}
	preset.Found = true
	preset.SourceCurrency = currency
	preset.SettlementCurrency = "USD"
	preset.UnitPolicy = "nominal"
	return preset, true
}
