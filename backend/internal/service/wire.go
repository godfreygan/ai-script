package service

import "github.com/google/wire"

// ProviderSet exposes service constructors for Wire.
var ProviderSet = wire.NewSet(
	NewServices,
)
