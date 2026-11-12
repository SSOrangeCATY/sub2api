# Model Pricing Data

This directory contains a local copy of the mirrored model pricing data as a fallback mechanism.

## Source
The original file is maintained by the LiteLLM project and mirrored into the `price-mirror` branch of this repository via GitHub Actions:
- Mirror branch (configurable via `PRICE_MIRROR_REPO`): https://raw.githubusercontent.com/<your-repo>/price-mirror/model_prices_and_context_window.json
- Upstream source: https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json

## Purpose
This local copy serves as a fallback when the remote file cannot be downloaded due to:
- Network restrictions
- Firewall rules
- DNS resolution issues
- GitHub being blocked in certain regions
- Docker container network limitations

## Update Process
The pricingService will:
1. First attempt to download the latest version from GitHub
2. Merge models missing from the downloaded catalog from this local fallback
3. If download fails, load this local copy instead
4. Apply an explicitly configured override file at the highest catalog priority

## GPT-6.2 default card (2026-11-12)

The operator supplied a shared USD card for `gpt-6.2-astra`, `gpt-6.2-sol`,
`gpt-6.2-astra-pro` and `gpt-6.2-sol-pro`: input $5, output $30 and cached input
$0.50 per million tokens. Their entries prevent a missing catalog key from
falling through to an older GPT model's prices. This is an operator-provided
default, not a claim of independently verified provider pricing.

The same card is used by the narrow static fallbacks if the dynamic pricing
service or its fallback file is unavailable. Explicit remote/override entries
and group/channel pricing keep their existing priority. No long-context
threshold or additional service-tier rate is inferred from the supplied base
card. Zero image fields retain the existing unconfigured-field semantics;
they do not introduce free image-token billing. Previous model entries and
historical usage records are unchanged.

## Manual Update
To manually update this file with the latest pricing data (if automation is unavailable):
```bash
curl -s https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json -o model_prices_and_context_window.json
```

## File Format
The file contains JSON data with model pricing information including:
- Model names and identifiers
- Input/output token costs
- Context window sizes
- Model capabilities

Last updated: 2026-11-12
