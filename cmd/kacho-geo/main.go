// Command kacho-geo — gRPC control-plane for Geography (Region / Zone).
//
// Leaf platform-topology service (depends on nothing else in Kachō by build;
// at runtime it CONSUMES kacho-iam authz Check). Public :9090 read-only
// (RegionService/ZoneService Get/List); cluster-internal :9091 admin CRUD
// (InternalRegion/ZoneService) — never on the external TLS endpoint (ban #6).
package main

import (
	"log"
	"os"

	"github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/config"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: kacho-geo {serve}")
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	switch os.Args[1] {
	case "serve":
		if err := runServe(cfg); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown command: %s (migrations: use the kacho-migrator binary)", os.Args[1])
	}
}
