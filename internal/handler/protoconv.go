// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package handler

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	geov1 "github.com/PRO-Robotech/kacho-proto/gen/go/kacho/cloud/geo/v1"

	"github.com/PRO-Robotech/kacho-geo/internal/domain"
)

// toProtoRegion конвертирует domain.Region → geov1.Region (created_at
// усекается до секунд).
func toProtoRegion(r *domain.Region) *geov1.Region {
	return &geov1.Region{
		Id:        r.ID,
		Name:      r.Name,
		CreatedAt: ts(r.CreatedAt),
	}
}

// toProtoZone конвертирует domain.Zone → geov1.Zone.
func toProtoZone(z *domain.Zone) *geov1.Zone {
	return &geov1.Zone{
		Id:        z.ID,
		RegionId:  z.RegionID,
		Status:    geov1.Zone_Status(z.Status),
		Name:      z.Name,
		CreatedAt: ts(z.CreatedAt),
	}
}

func ts(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t.Truncate(time.Second))
}
