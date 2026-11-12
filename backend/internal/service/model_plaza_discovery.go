package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

// Channel catalogs stay authoritative. Groups served directly by accounts use
// configured public model IDs, with provider defaults for unmapped accounts.
// This reads one persistent candidate snapshot, never probes an upstream, and
// never exports account IDs, mapping targets or credentials.
func (s *ModelPlazaService) fillNativeGroupModels(ctx context.Context, groups map[int64]*PlazaGroup, entities map[int64]*Group, channelBound map[int64]bool) error {
	if s.accountRepo == nil {
		return nil
	}
	needed := false
	for id := range groups {
		needed = needed || !channelBound[id]
	}
	if !needed {
		return nil
	}
	accounts, err := s.accountRepo.ListModelAvailabilityCandidates(ctx, nil, domain.ConcretePlatformIDs(), true)
	if err != nil {
		return fmt.Errorf("list native model catalog accounts: %w", err)
	}
	seen := map[int64]map[string]bool{}
	for i := range accounts {
		a := &accounts[i]
		if a.Status != StatusActive || !a.Schedulable {
			continue
		}
		for _, gid := range a.GroupIDs {
			g := groups[gid]
			if g == nil || channelBound[gid] {
				continue
			}
			platform := a.Platform
			if g.Platform != PlatformComposite && platform != g.Platform {
				if a.Platform != PlatformAntigravity || !a.IsMixedSchedulingEnabled() || (g.Platform != PlatformAnthropic && g.Platform != PlatformGemini) {
					continue
				}
				platform = g.Platform
			}
			if seen[gid] == nil {
				seen[gid] = map[string]bool{}
			}
			for _, name := range nativePlazaModelIDs(a, entities[gid]) {
				if a.Platform != platform {
					// Public aliases need not contain a provider's name. Inspect
					// the private target only to select its request platform.
					target := mapAntigravityModel(a, name)
					if (platform == PlatformGemini && !isAntigravityGeminiModel(target)) || (platform == PlatformAnthropic && !strings.HasPrefix(strings.ToLower(target), "claude-")) {
						continue
					}
				}
				key := platform + "\x00" + name
				if seen[gid][key] || !(&GatewayService{}).isModelSupportedByAccountWithContext(ctx, a, name) {
					continue
				}
				seen[gid][key] = true
				g.Models = append(g.Models, PlazaModel{Name: name, Platform: platform})
			}
		}
	}
	return nil
}

func nativePlazaModelIDs(account *Account, group *Group) []string {
	mapping := account.GetModelMapping()
	names := map[string]bool{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name != "" && !strings.Contains(name, "*") {
			names[name] = true
		}
	}
	for name, target := range mapping {
		if strings.TrimSpace(target) != "" {
			add(name)
		}
	}
	if group != nil {
		for _, pricing := range group.ModelPricing {
			if pricing.Platform == "" || pricing.Platform == account.Platform {
				for _, name := range pricing.Models {
					add(name)
				}
			}
		}
	}
	// Expand wildcard mappings only against the existing provider catalog.
	// Exact mapping targets are upstream-private names and are never listed.
	var defaults []string
	switch account.Platform {
	case PlatformOpenAI:
		defaults = openai.DefaultModelIDs()
	case PlatformAnthropic:
		defaults = claude.DefaultModelIDs()
	case PlatformGemini:
		for _, model := range geminicli.DefaultModels {
			defaults = append(defaults, model.ID)
		}
	case PlatformDeepseek:
		defaults = deepseekServableModels
	}
	for _, name := range defaults {
		if len(mapping) == 0 || mappingSupportsRequestedModel(mapping, name) {
			add(name)
		}
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	return out
}
