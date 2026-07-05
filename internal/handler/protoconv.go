// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package handler

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/PRO-Robotech/kacho-corelib/operations"
	operationpb "github.com/PRO-Robotech/kacho-proto/gen/go/kacho/cloud/operation"
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

// operationToProto конвертирует corelib operations.Operation в proto-форму
// (OperationService.Get/мутации возвращают её клиенту). oneof result —
// error|response (заполнен только при done).
func operationToProto(op *operations.Operation) *operationpb.Operation {
	if op == nil {
		return nil
	}
	p := &operationpb.Operation{
		Id:                   op.ID,
		Description:          op.Description,
		CreatedAt:            timestamppb.New(op.CreatedAt),
		CreatedBy:            op.CreatedBy,
		ModifiedAt:           timestamppb.New(op.ModifiedAt),
		Done:                 op.Done,
		Metadata:             op.Metadata,
		PrincipalType:        op.Principal.Type,
		PrincipalId:          op.Principal.ID,
		PrincipalDisplayName: op.Principal.DisplayName,
	}
	if op.Error != nil {
		p.Result = &operationpb.Operation_Error{Error: op.Error}
	} else if op.Response != nil {
		p.Result = &operationpb.Operation_Response{Response: op.Response}
	}
	return p
}
