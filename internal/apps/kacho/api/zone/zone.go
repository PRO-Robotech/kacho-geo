// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

// Package zone — use-case (бизнес-логика) каталога Zone.
//
// Use-case слой чистой архитектуры: импортирует domain + порт ZoneRepo + corelib
// operations, не тянет pgx/transport. Публичные ZoneService.Get/List — read-only
// (sync). Admin CRUD идет через InternalZoneService на :9091 и возвращает
// Operation (async LRO): мутация синхронно отдает operation.Operation
// (done=false), фоновый corelib-worker выполняет доменную запись и финализирует
// операцию (response=Zone либо Empty для Delete, либо error). Клиент поллит
// OperationService.Get(id) до done.
package zone

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/PRO-Robotech/kacho-corelib/operations"
	"github.com/PRO-Robotech/kacho-corelib/validate"
	geov1 "github.com/PRO-Robotech/kacho-proto/gen/go/kacho/cloud/geo/v1"

	"github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/shared/lro"
	"github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/shared/serviceerr"
	"github.com/PRO-Robotech/kacho-geo/internal/domain"
	geoerrors "github.com/PRO-Robotech/kacho-geo/internal/errors"
)

// Pagination — вход для List с cursor-пагинацией (page_size + opaque page_token).
type Pagination struct {
	PageSize  int64
	PageToken string
}

// UpdateParams — опциональные поля partial-Update зоны. nil → поле не меняется
// (repo делает COALESCE-апдейт). Позволяет атомарный single-statement UPDATE без
// предварительного Get (исключен TOCTOU read-modify-write).
type UpdateParams struct {
	RegionID *string
	Name     *string
	Status   *domain.ZoneStatus
}

// Repo — port-интерфейс к таблице zones (read + admin CRUD). Реализуется
// internal/repo/kacho/pg.ZoneRepo; для unit-тестов подменяется repomock.
//
// Update — атомарный single-statement (UPDATE … RETURNING) по UpdateParams;
// 0 rows из RETURNING → ErrNotFound.
type Repo interface {
	Get(ctx context.Context, id string) (*domain.Zone, error)
	List(ctx context.Context, p Pagination) ([]*domain.Zone, string, error)
	Insert(ctx context.Context, z *domain.Zone) (*domain.Zone, error)
	Update(ctx context.Context, id string, p UpdateParams) (*domain.Zone, error)
	Delete(ctx context.Context, id string) error
}

// UseCase — бизнес-логика Zone поверх порта Repo и LRO-стека operations.
type UseCase struct {
	repo Repo
	ops  operations.Repo
}

// New собирает UseCase для Zone. ops — corelib LRO-репозиторий operations-таблицы.
func New(repo Repo, ops operations.Repo) *UseCase { return &UseCase{repo: repo, ops: ops} }

// Get возвращает Zone по id.
func (u *UseCase) Get(ctx context.Context, id string) (*domain.Zone, error) {
	if id == "" {
		return nil, geoerrors.ErrInvalidArg
	}
	z, err := u.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return z, nil
}

// List возвращает зоны (cursor-пагинация по id; garbage page_size → InvalidArgument).
func (u *UseCase) List(ctx context.Context, p Pagination) ([]*domain.Zone, string, error) {
	size, err := validate.PageSize("page_size", p.PageSize)
	if err != nil {
		return nil, "", err
	}
	p.PageSize = size
	return u.repo.List(ctx, p)
}

// Create принимает запрос на создание Zone (admin-only) и возвращает Operation.
// Малформ/невалидный вход (пустой id/region_id, длинное name, out-of-range
// status) отвергается СИНХРОННО (InvalidArgument). Несуществующий region_id —
// FK-нарушение на вставке → Operation.error FailedPrecondition (источник истины
// DB, не software-precheck).
func (u *UseCase) Create(ctx context.Context, id, regionID, name string, st domain.ZoneStatus) (*operations.Operation, error) {
	z := domain.Zone{ID: id, RegionID: regionID, Name: name, Status: st}
	if err := z.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", geoerrors.ErrInvalidArg, err.Error())
	}
	op, err := operations.NewFromContext(ctx, lro.OperationPrefix,
		fmt.Sprintf("Create zone %s", id),
		&geov1.CreateZoneMetadata{ZoneId: id})
	if err != nil {
		return nil, err
	}
	if err := u.ops.Create(ctx, op); err != nil {
		return nil, err
	}
	operations.Run(ctx, u.ops, op.ID, func(ctx context.Context) (*anypb.Any, error) {
		created, derr := u.repo.Insert(ctx, &z)
		if derr != nil {
			return nil, serviceerr.ToStatus(derr)
		}
		return marshalZone(created)
	})
	return &op, nil
}

// Update принимает запрос на partial-смену Zone (admin-only) и возвращает
// Operation. Пустой id → синхронный InvalidArgument. Пустые regionID/name и
// unspecified-status НЕ меняют поле (nil → COALESCE в repo). not-found/конфликт →
// Operation.error.
func (u *UseCase) Update(ctx context.Context, id, regionID, name string, st domain.ZoneStatus) (*operations.Operation, error) {
	if id == "" {
		return nil, geoerrors.ErrInvalidArg
	}
	var p UpdateParams
	if regionID != "" {
		p.RegionID = &regionID
	}
	if name != "" {
		if err := domain.ValidateName("zone name", name); err != nil {
			return nil, fmt.Errorf("%w: %s", geoerrors.ErrInvalidArg, err.Error())
		}
		p.Name = &name
	}
	if st != domain.ZoneStatusUnspecified {
		if err := st.Validate(); err != nil {
			return nil, fmt.Errorf("%w: %s", geoerrors.ErrInvalidArg, err.Error())
		}
		p.Status = &st
	}
	op, err := operations.NewFromContext(ctx, lro.OperationPrefix,
		fmt.Sprintf("Update zone %s", id),
		&geov1.UpdateZoneMetadata{ZoneId: id})
	if err != nil {
		return nil, err
	}
	if err := u.ops.Create(ctx, op); err != nil {
		return nil, err
	}
	operations.Run(ctx, u.ops, op.ID, func(ctx context.Context) (*anypb.Any, error) {
		updated, derr := u.repo.Update(ctx, id, p)
		if derr != nil {
			return nil, serviceerr.ToStatus(derr)
		}
		return marshalZone(updated)
	})
	return &op, nil
}

// Delete принимает запрос на удаление Zone (admin-only) и возвращает Operation.
// Пустой id → синхронный InvalidArgument. not-found → Operation.error NotFound.
// Успех → response=Empty.
func (u *UseCase) Delete(ctx context.Context, id string) (*operations.Operation, error) {
	if id == "" {
		return nil, geoerrors.ErrInvalidArg
	}
	op, err := operations.NewFromContext(ctx, lro.OperationPrefix,
		fmt.Sprintf("Delete zone %s", id),
		&geov1.DeleteZoneMetadata{ZoneId: id})
	if err != nil {
		return nil, err
	}
	if err := u.ops.Create(ctx, op); err != nil {
		return nil, err
	}
	operations.Run(ctx, u.ops, op.ID, func(ctx context.Context) (*anypb.Any, error) {
		if derr := u.repo.Delete(ctx, id); derr != nil {
			return nil, serviceerr.ToStatus(derr)
		}
		return anypb.New(&emptypb.Empty{})
	})
	return &op, nil
}

// marshalZone упаковывает domain.Zone в Operation.response (geov1.Zone,
// created_at усекается до секунд).
func marshalZone(z *domain.Zone) (*anypb.Any, error) {
	return anypb.New(&geov1.Zone{
		Id:        z.ID,
		RegionId:  z.RegionID,
		Status:    geov1.Zone_Status(z.Status),
		Name:      z.Name,
		CreatedAt: timestamppb.New(z.CreatedAt.Truncate(time.Second)),
	})
}
