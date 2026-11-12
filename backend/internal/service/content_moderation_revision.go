package service

// ContentModerationConfigRevision shares the configuration reader's treatment
// of empty legacy documents and missing or zero revisions.
func ContentModerationConfigRevision(raw string) (int64, error) {
	cfg, err := parseContentModerationConfig(raw)
	if err != nil {
		return 0, err
	}
	return cfg.Revision, nil
}
