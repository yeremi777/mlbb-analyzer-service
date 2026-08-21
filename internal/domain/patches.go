package domain

import "time"

// Patch is one released game version and the day it shipped.
//
// ReleaseDate identifies a patch, not Version: patches before 2025 reused a
// version number across successive releases (1.9.42 shipped five times), while
// every release date is distinct.
type Patch struct {
	Version     string
	ReleaseDate time.Time
}
