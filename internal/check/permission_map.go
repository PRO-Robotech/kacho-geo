package check

import (
	"github.com/PRO-Robotech/kacho-corelib/authz"
)

// FGA scoping for kacho-geo: Region/Zone are global cluster-scoped read-only
// catalogs. Public Get/List are gated viewer-tier on the cluster singleton;
// admin Internal* CRUD is gated system_admin on the same singleton (mirrors the
// geo.v1 proto annotations: required_relation viewer / system_admin,
// scope_extractor.object_type=cluster). cluster_kacho_root is the kacho-iam
// ClusterSingletonID — one per deploy.
const (
	objectTypeCluster      = "cluster"
	clusterSingletonObject = "cluster_kacho_root"

	relationViewer      = "viewer"
	relationSystemAdmin = "system_admin"
)

// staticClusterCatalog — extractor that always returns (cluster, cluster_kacho_root).
func staticClusterCatalog() authz.ObjectExtractor {
	return func(req any) (string, string, error) {
		return objectTypeCluster, clusterSingletonObject, nil
	}
}

// PermissionMap maps each geo RPC → required relation + object extractor.
//
// Public read (Region/Zone Get/List) → viewer on cluster:cluster_kacho_root.
// Admin Internal* CRUD → system_admin on cluster:cluster_kacho_root.
//
// Permission strings use the corrected geo.* FQN — the compute typo
// `regionses`/`zoneses` is intentionally fixed to `geo.regions.list` /
// `geo.zones.list` (acceptance scenario 6.0-01). The interceptor currently gates
// on Relation; Permission is carried for the future fine-grained model + parity
// with the api-gateway permission_catalog.
func PermissionMap() authz.RPCMap {
	return authz.RPCMap{
		// ---- public read-only catalog (viewer floor) ----
		"/kacho.cloud.geo.v1.RegionService/Get": {
			Relation:   relationViewer,
			Extract:    staticClusterCatalog(),
			Permission: "geo.regions.get",
		},
		"/kacho.cloud.geo.v1.RegionService/List": {
			Relation:   relationViewer,
			Extract:    staticClusterCatalog(),
			Permission: "geo.regions.list",
		},
		"/kacho.cloud.geo.v1.ZoneService/Get": {
			Relation:   relationViewer,
			Extract:    staticClusterCatalog(),
			Permission: "geo.zones.get",
		},
		"/kacho.cloud.geo.v1.ZoneService/List": {
			Relation:   relationViewer,
			Extract:    staticClusterCatalog(),
			Permission: "geo.zones.list",
		},

		// ---- admin CRUD (Internal*, system_admin) ----
		"/kacho.cloud.geo.v1.InternalRegionService/Create": {
			Relation:   relationSystemAdmin,
			Extract:    staticClusterCatalog(),
			Permission: "geo.regions.create",
		},
		"/kacho.cloud.geo.v1.InternalRegionService/Update": {
			Relation:   relationSystemAdmin,
			Extract:    staticClusterCatalog(),
			Permission: "geo.regions.update",
		},
		"/kacho.cloud.geo.v1.InternalRegionService/Delete": {
			Relation:   relationSystemAdmin,
			Extract:    staticClusterCatalog(),
			Permission: "geo.regions.delete",
		},
		"/kacho.cloud.geo.v1.InternalZoneService/Create": {
			Relation:   relationSystemAdmin,
			Extract:    staticClusterCatalog(),
			Permission: "geo.zones.create",
		},
		"/kacho.cloud.geo.v1.InternalZoneService/Update": {
			Relation:   relationSystemAdmin,
			Extract:    staticClusterCatalog(),
			Permission: "geo.zones.update",
		},
		"/kacho.cloud.geo.v1.InternalZoneService/Delete": {
			Relation:   relationSystemAdmin,
			Extract:    staticClusterCatalog(),
			Permission: "geo.zones.delete",
		},
	}
}
