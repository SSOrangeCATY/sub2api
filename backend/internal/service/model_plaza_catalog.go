package service

// channelHasSalesCatalog distinguishes public model catalogs from channels
// retained solely for account procurement cost rules.
func channelHasSalesCatalog(channel *Channel) bool {
	if channel == nil {
		return false
	}
	return len(channel.ModelPricing) > 0 || len(channel.ModelMapping) > 0 || channel.RestrictModels
}
