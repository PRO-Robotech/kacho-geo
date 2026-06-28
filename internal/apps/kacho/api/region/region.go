// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

// Package region — use-case (бизнес-логика) каталога Region.
//
// Use-case слой чистой архитектуры: импортирует domain + порт RegionRepo +
// corelib operations, не тянет pgx/transport. Публичные RegionService.Get/List —
// read-only (sync). Admin CRUD (Create/Update/Delete) идет через
// InternalRegionService на :9091 и возвращает Operation (async LRO): мутация
// синхронно отдает operation.Operation (done=false), фоновый corelib-worker
// выполняет доменную запись и финализирует операцию (done=true, response=Region
// либо Empty для Delete, либо error=google.rpc.Status). Клиент поллит
// OperationService.Get(id) до done.
package region

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/PRO-Robotech/kacho-corelib/operations"
	"github.com/PRO-Robotech/kacho-corelib/validate"
	geov1 "github.com/PRO-Robotech/kacho-geo/proto/gen/go/kacho/cloud/geo/v1"

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

// Repo — port-интерфейс к таблице regions (read + admin CRUD). Реализуется
// internal/repo/kacho/pg.RegionRepo; для unit-тестов подменяется repomock.
//
// Update — атомарный single-statement (UPDATE … RETURNING), без предварительного
// Get (исключен TOCTOU read-modify-write). name=nil → поле не меняется (COALESCE);
// 0 rows из RETURNING → ErrNotFound.
type Repo interface {
	Get(ctx context.Context, id string) (*domain.Region, error)
	List(ctx context.Context, p Pagination) ([]*domain.Region, string, error)
	Insert(ctx context.Context, r *domain.Region) (*domain.Region, error)
	Update(ctx context.Context, id string, name *string) (*domain.Region, error)
	Delete(ctx context.Context, id string) error
}

// UseCase — бизнес-логика Region поверх порта Repo и LRO-стека operations.
type UseCase struct {
	repo Repo
	ops  operations.Repo
}

// New собирает UseCase для Region. ops — corelib LRO-репозиторий operations-таблицы
// (admin-мутации async, см. doc пакета).
func New(repo Repo, ops operations.Repo) *UseCase { return &UseCase{repo: repo, ops: ops} }

// Get возвращает Region по id.
func (u *UseCase) Get(ctx context.Context, id string) (*domain.Region, error) {
	if id == "" {
		return nil, geoerrors.ErrInvalidArg
	}
	r, err := u.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// List возвращает регионы (cursor-пагинация по id; garbage page_size → InvalidArgument).
func (u *UseCase) List(ctx context.Context, p Pagination) ([]*domain.Region, string, error) {
	size, err := validate.PageSize("page_size", p.PageSize)
	if err != nil {
		return nil, "", err
	}
	p.PageSize = size
	return u.repo.List(ctx, p)
}

// Create принимает запрос на создание Region (admin-only) и возвращает Operation.
// Малформ/невалидный вход (пустой id, слишком длинное name) отвергается
// СИНХРОННО (InvalidArgument) — операция в таблицу не пишется. Валидный вход →
// LRO-строка (done=false) + фоновый worker, который вставляет регион и
// финализирует операцию (response=Region или error).
func (u *UseCase) Create(ctx context.Context, id, name string) (*operations.Operation, error) {
	r := domain.Region{ID: id, Name: name}
	if err := r.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", geoerrors.ErrInvalidArg, err.Error())
	}
	op, err := operations.NewFromContext(ctx, lro.OperationPrefix,
		fmt.Sprintf("Create region %s", id),
		&geov1.CreateRegionMetadata{RegionId: id})
	if err != nil {
		return nil, err
	}
	if err := u.ops.Create(ctx, op); err != nil {
		return nil, err
	}
	operations.Run(ctx, u.ops, op.ID, func(ctx context.Context) (*anypb.Any, error) {
		created, derr := u.repo.Insert(ctx, &r)
		if derr != nil {
			return nil, serviceerr.ToStatus(derr)
		}
		return marshalRegion(created)
	})
	return &op, nil
}

// Update принимает запрос на смену name у Region (admin-only) и возвращает
// Operation. Пустой id → синхронный InvalidArgument. name="" → поле не меняется
// (nil в repo). Доменная запись и not-found/конфликт → в Operation.error.
func (u *UseCase) Update(ctx context.Context, id, name string) (*operations.Operation, error) {
	if id == "" {
		return nil, geoerrors.ErrInvalidArg
	}
	var namePtr *string
	if name != "" {
		if err := domain.ValidateName("region name", name); err != nil {
			return nil, fmt.Errorf("%w: %s", geoerrors.ErrInvalidArg, err.Error())
		}
		namePtr = &name
	}
	op, err := operations.NewFromContext(ctx, lro.OperationPrefix,
		fmt.Sprintf("Update region %s", id),
		&geov1.UpdateRegionMetadata{RegionId: id})
	if err != nil {
		return nil, err
	}
	if err := u.ops.Create(ctx, op); err != nil {
		return nil, err
	}
	operations.Run(ctx, u.ops, op.ID, func(ctx context.Context) (*anypb.Any, error) {
		updated, derr := u.repo.Update(ctx, id, namePtr)
		if derr != nil {
			return nil, serviceerr.ToStatus(derr)
		}
		return marshalRegion(updated)
	})
	return &op, nil
}

// Delete принимает запрос на удаление Region (admin-only) и возвращает Operation.
// Пустой id → синхронный InvalidArgument. Блокировка FK RESTRICT (есть зоны) и
// not-found → в Operation.error (FailedPrecondition / NotFound). Успех →
// response=Empty (без тела ресурса).
func (u *UseCase) Delete(ctx context.Context, id string) (*operations.Operation, error) {
	if id == "" {
		return nil, geoerrors.ErrInvalidArg
	}
	op, err := operations.NewFromContext(ctx, lro.OperationPrefix,
		fmt.Sprintf("Delete region %s", id),
		&geov1.DeleteRegionMetadata{RegionId: id})
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

// marshalRegion упаковывает domain.Region в Operation.response (geov1.Region,
// created_at усекается до секунд — единый формат timestamp Kachō).
func marshalRegion(r *domain.Region) (*anypb.Any, error) {
	return anypb.New(&geov1.Region{
		Id:        r.ID,
		Name:      r.Name,
		CreatedAt: timestamppb.New(r.CreatedAt.Truncate(time.Second)),
	})
}
