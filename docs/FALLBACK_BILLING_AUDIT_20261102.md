# Fallback billing audit — 2026-11-02

## Scope and expected behavior

- A fallback entry with NULL fallback_rate_multiplier inherits the selected target group's base text multiplier on each new request. An explicit positive entry value overrides it. Clearing the field restores inheritance; zero is not a clearing value.
- The selected target's pricing cards, quotas, RPM and usage attribution remain authoritative. Purchase billing identity uses the actual selected account cost without adding the entry or target sales multiplier.
- Route information, API Key displays, billing introspection, reservations, profit checks and actual billing use the same rule. Missing routes return unavailable rather than a fabricated multiplier.
- Create, edit and duplicate preserve nullable configuration; omitted updates retain it and explicit null clears it. Authentication snapshot version is bumped.

## Review and corrections

Independent read-only review found no remaining P1/P2 issues in the final change. Earlier findings were fixed: duplicate-group configuration, the API Key group-change popover and purchase-user route labels.

The billing review also corrected one-hour cache-write cost, preserved additive search charges exactly once through account-cost overrides and accepted explicitly configured zero purchase prices. Unconfigured prices remain unknown.

## Verification

- Frontend type checking passed. The final critical regression suite passed 629 tests across 38 files; changed frontend files passed ESLint. Additional admin view tests passed.
- Backend targeted service, handler, admin, DTO, middleware, repository, server and migration tests passed, including nullable input boundaries, cached-rate bypass, target switches, overrides, clearing, target repricing, request reservations, actual cost and purchase identity.
- The complete backend run did not pass: the service package reported TestOllamaProbeCallback_StaleLongDoesNotOverrideNewShort. The same failure was reproduced in an isolated archive of pre-patch commit e3ebeee with a 20-run command, confirming it predates this patch. The unchanged Ollama test/implementation is outside this billing patch; the backend suite is not reported as fully passing.
- The PostgreSQL dumper tests passed with Git sh available on the Windows PATH.
- Migration 252 was checked statically. Real PostgreSQL migration execution, browser visual acceptance and production behavior have not been verified in this audit.

## Release boundary and DeepSeek configuration

The patch is reviewed through fix/deepseek-billing-20261102 → custom/main. No merge to production, production image build, deployment or live data migration is part of this audit.

After an approved release and migration 252, locate the actual DeepSeek fallback entry, explicitly save fallback_rate_multiplier as null, read it back and verify its displayed target and effective multiplier against the target group. Existing fallback rows receive NULL by default. Never replace the legacy required rate_multiplier with zero to imitate clearing. The live DeepSeek configuration has not been changed by this patch.

The separately authorized purchase-identity migration and retirement of personal rates also remain unexecuted in production.
