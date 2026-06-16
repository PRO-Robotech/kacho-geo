// Package domain — entities for kacho-geo (Geography: Region / Zone).
//
// Clean-arch domain layer: pure Go (stdlib only). Region/Zone are global
// platform-topology resources owned by kacho-geo (the leaf service). They are
// NOT bound to Project/Account — cluster-scoped topology. Other services
// reference a region/zone by id (string, no cross-service FK) and validate via
// RegionService.Get / ZoneService.Get.
package domain

import (
	"fmt"
	"time"
)

// ZoneStatus — availability-zone status. Width int32 matches geov1.Zone_Status
// exactly, so the domain↔proto conversions are exact (no int→int32 narrowing).
type ZoneStatus int32

// ZoneStatus values (parity with geo.v1 proto enum: UNSPECIFIED=0, UP=1, DOWN=2).
const (
	ZoneStatusUnspecified ZoneStatus = iota
	ZoneStatusUp
	ZoneStatusDown
)

// Region — global geography resource (id = "ru-central1"). Admin-assigned,
// immutable PK. Domain kacho-geo (ported from kacho-compute, epic kacho-geo S2).
type Region struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// Validate checks domain invariants for a Region intended to be created/stored.
// id must be non-empty (admin-assigned PK). Returns nil when valid.
func (r Region) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("region id is required")
	}
	return nil
}

// Zone — availability-zone (global read-only catalog; id = "ru-central1-a").
// Belongs to a Region (region_id, FK RESTRICT on the DB side).
type Zone struct {
	ID        string
	RegionID  string
	Name      string
	Status    ZoneStatus
	CreatedAt time.Time
}

// Validate checks domain invariants for a Zone intended to be created/stored.
// id and region_id must be non-empty. Returns nil when valid.
func (z Zone) Validate() error {
	if z.ID == "" {
		return fmt.Errorf("zone id is required")
	}
	if z.RegionID == "" {
		return fmt.Errorf("zone region_id is required")
	}
	return nil
}
