package check

import "testing"

func TestPermissionMap_tiersAndPermissions(t *testing.T) {
	m := PermissionMap()

	read := []struct{ method, perm string }{
		{"/kacho.cloud.geo.v1.RegionService/Get", "geo.regions.get"},
		{"/kacho.cloud.geo.v1.RegionService/List", "geo.regions.list"},
		{"/kacho.cloud.geo.v1.ZoneService/Get", "geo.zones.get"},
		{"/kacho.cloud.geo.v1.ZoneService/List", "geo.zones.list"},
	}
	for _, r := range read {
		e, ok := m.Lookup(r.method)
		if !ok {
			t.Fatalf("%s missing from PermissionMap", r.method)
		}
		if e.Relation != relationViewer {
			t.Errorf("%s relation = %q, want viewer", r.method, e.Relation)
		}
		if e.Permission != r.perm {
			t.Errorf("%s permission = %q, want %q", r.method, e.Permission, r.perm)
		}
	}

	admin := []struct{ method, perm string }{
		{"/kacho.cloud.geo.v1.InternalRegionService/Create", "geo.regions.create"},
		{"/kacho.cloud.geo.v1.InternalRegionService/Update", "geo.regions.update"},
		{"/kacho.cloud.geo.v1.InternalRegionService/Delete", "geo.regions.delete"},
		{"/kacho.cloud.geo.v1.InternalZoneService/Create", "geo.zones.create"},
		{"/kacho.cloud.geo.v1.InternalZoneService/Update", "geo.zones.update"},
		{"/kacho.cloud.geo.v1.InternalZoneService/Delete", "geo.zones.delete"},
	}
	for _, a := range admin {
		e, ok := m.Lookup(a.method)
		if !ok {
			t.Fatalf("%s missing from PermissionMap", a.method)
		}
		if e.Relation != relationSystemAdmin {
			t.Errorf("%s relation = %q, want system_admin", a.method, e.Relation)
		}
		if e.Permission != a.perm {
			t.Errorf("%s permission = %q, want %q", a.method, e.Permission, a.perm)
		}
	}

	// Guard against the compute typo regression: no permission string contains "es.list".
	for method, e := range m {
		if e.Permission == "geo.regionses.list" || e.Permission == "geo.zoneses.list" {
			t.Errorf("%s carries the regionses/zoneses typo: %q", method, e.Permission)
		}
	}

	// Every entry must resolve to (cluster, cluster_kacho_root).
	for method, e := range m {
		ot, oid, err := e.Extract(nil)
		if err != nil {
			t.Fatalf("%s extract err = %v", method, err)
		}
		if ot != objectTypeCluster || oid != clusterSingletonObject {
			t.Errorf("%s extract = (%s,%s), want (cluster,cluster_kacho_root)", method, ot, oid)
		}
	}
}
