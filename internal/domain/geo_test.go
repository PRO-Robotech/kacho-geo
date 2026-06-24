// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package domain_test

import (
	"strings"
	"testing"

	"github.com/PRO-Robotech/kacho-geo/internal/domain"
)

func TestRegionValidate(t *testing.T) {
	longName := strings.Repeat("a", 254)
	tests := []struct {
		name    string
		region  domain.Region
		wantErr bool
	}{
		{name: "valid", region: domain.Region{ID: "region-1", Name: "Region 1"}, wantErr: false},
		{name: "valid no name", region: domain.Region{ID: "region-1"}, wantErr: false},
		{name: "empty id", region: domain.Region{Name: "x"}, wantErr: true},
		{name: "name too long", region: domain.Region{ID: "region-1", Name: longName}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.region.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestZoneValidate(t *testing.T) {
	longName := strings.Repeat("a", 254)
	tests := []struct {
		name    string
		zone    domain.Zone
		wantErr bool
	}{
		{name: "valid", zone: domain.Zone{ID: "region-1-a", RegionID: "region-1", Status: domain.ZoneStatusUp}, wantErr: false},
		{name: "valid down", zone: domain.Zone{ID: "region-1-a", RegionID: "region-1", Status: domain.ZoneStatusDown}, wantErr: false},
		{name: "valid unspecified", zone: domain.Zone{ID: "region-1-a", RegionID: "region-1", Status: domain.ZoneStatusUnspecified}, wantErr: false},
		{name: "empty id", zone: domain.Zone{RegionID: "region-1"}, wantErr: true},
		{name: "empty region_id", zone: domain.Zone{ID: "region-1-a"}, wantErr: true},
		{name: "name too long", zone: domain.Zone{ID: "region-1-a", RegionID: "region-1", Name: longName, Status: domain.ZoneStatusUp}, wantErr: true},
		{name: "status out of range", zone: domain.Zone{ID: "region-1-a", RegionID: "region-1", Status: domain.ZoneStatus(99)}, wantErr: true},
		{name: "status negative", zone: domain.Zone{ID: "region-1-a", RegionID: "region-1", Status: domain.ZoneStatus(-1)}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.zone.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestZoneStatusValues(t *testing.T) {
	// Parity с нумерацией proto-enum geo.v1.
	if domain.ZoneStatusUnspecified != 0 || domain.ZoneStatusUp != 1 || domain.ZoneStatusDown != 2 {
		t.Fatalf("ZoneStatus enum values drifted from proto (UNSPECIFIED=0, UP=1, DOWN=2)")
	}
}
