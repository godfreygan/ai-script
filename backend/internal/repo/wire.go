package repo

import "github.com/google/wire"

// ProviderSet exposes repository constructors for Wire.
var ProviderSet = wire.NewSet(
	NewRepositories,
)
