package handler

import "github.com/google/wire"

// ProviderSet exposes handler constructors for Wire.
var ProviderSet = wire.NewSet(
	NewHandlers,
)
