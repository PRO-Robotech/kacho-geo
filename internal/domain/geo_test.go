package domain_test

import (
	"testing"

	"github.com/PRO-Robotech/kacho-geo/internal/domain"
)

func TestRegionValidate(t *testing.T) {
	tests := []struct {
		name    string
		region  domain.Region
		wantErr bool
	}{
		{name: "valid", region: domain.Region{ID: "ru-central1", Name: "Russia Central 1"}, wantErr: false},
		{name: "valid no name", region: domain.Region{ID: "ru-central1"}, wantErr: false},
		{name: "empty id", region: domain.Region{Name: "x"}, wantErr: true},
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
	tests := []struct {
		name    string
		zone    domain.Zone
		wantErr bool
	}{
		{name: "valid", zone: domain.Zone{ID: "ru-central1-a", RegionID: "ru-central1", Status: domain.ZoneStatusUp}, wantErr: false},
		{name: "empty id", zone: domain.Zone{RegionID: "ru-central1"}, wantErr: true},
		{name: "empty region_id", zone: domain.Zone{ID: "ru-central1-a"}, wantErr: true},
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
	// Parity with geo.v1 proto enum numbering.
	if domain.ZoneStatusUnspecified != 0 || domain.ZoneStatusUp != 1 || domain.ZoneStatusDown != 2 {
		t.Fatalf("ZoneStatus enum values drifted from proto (UNSPECIFIED=0, UP=1, DOWN=2)")
	}
}
