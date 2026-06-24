// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

// Package region — use-case (бизнес-логика) каталога Region.
//
// Use-case слой чистой архитектуры: импортирует domain + порт RegionRepo, не тянет
// pgx/grpc-stubs/transport. Публичные RegionService.Get/List — read-only (sync);
// admin CRUD (Create/Update/Delete) идет через InternalRegionService на :9091 и
// возвращает ресурс синхронно (catalog-паттерн: admin-managed справочник с
// admin-assigned immutable id, осознанно не через Operation).
package region

import (
	"context"
	"fmt"

	"github.com/PRO-Robotech/kacho-corelib/validate"

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

// UseCase — бизнес-логика Region поверх порта Repo.
type UseCase struct {
	repo Repo
}

// New собирает UseCase для Region.
func New(repo Repo) *UseCase { return &UseCase{repo: repo} }

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
// Валидация/нормализация page_size — здесь (use-case владеет валидацией входа);
// repo получает уже нормализованный pageSize (0 → default, >max → InvalidArgument).
func (u *UseCase) List(ctx context.Context, p Pagination) ([]*domain.Region, string, error) {
	size, err := validate.PageSize("page_size", p.PageSize)
	if err != nil {
		return nil, "", err
	}
	p.PageSize = size
	return u.repo.List(ctx, p)
}

// Create вставляет Region (admin-only). id назначается админом и неизменяем.
func (u *UseCase) Create(ctx context.Context, id, name string) (*domain.Region, error) {
	r := domain.Region{ID: id, Name: name}
	if err := r.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", geoerrors.ErrInvalidArg, err.Error())
	}
	created, err := u.repo.Insert(ctx, &r)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// Update меняет name у Region (admin-only). id неизменяем. Атомарно через
// repo.Update (single-statement, без Get-then-Update / TOCTOU). name="" →
// поле не меняется (nil в repo). Перед записью валидируется новое имя.
func (u *UseCase) Update(ctx context.Context, id, name string) (*domain.Region, error) {
	if id == "" {
		return nil, geoerrors.ErrInvalidArg
	}
	var namePtr *string
	if name != "" {
		// Валидируем новое имя (Update раньше не валидировал, в отличие от Create).
		if err := domain.ValidateName("region name", name); err != nil {
			return nil, fmt.Errorf("%w: %s", geoerrors.ErrInvalidArg, err.Error())
		}
		namePtr = &name
	}
	updated, err := u.repo.Update(ctx, id, namePtr)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Delete удаляет Region (admin-only). Блокируется, пока на него ссылаются zone:
// источник истины — FK RESTRICT zones→regions на DB-уровне (SQLSTATE 23503 →
// ErrFailedPrecondition в repo.Delete), без software check-then-act / TOCTOU.
func (u *UseCase) Delete(ctx context.Context, id string) error {
	if id == "" {
		return geoerrors.ErrInvalidArg
	}
	if err := u.repo.Delete(ctx, id); err != nil {
		return err
	}
	return nil
}
