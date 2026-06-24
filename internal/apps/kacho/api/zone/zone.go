// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

// Package zone — use-case (бизнес-логика) каталога Zone.
//
// Use-case слой чистой архитектуры: импортирует domain + порт ZoneRepo, не тянет
// pgx/grpc-stubs/transport. Публичные ZoneService.Get/List — read-only (sync);
// admin CRUD идет через InternalZoneService на :9091 и возвращает ресурс синхронно
// (catalog-паттерн: admin-managed справочник с admin-assigned immutable id,
// осознанно не через Operation).
package zone

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

// UseCase — бизнес-логика Zone поверх порта Repo.
type UseCase struct {
	repo Repo
}

// New собирает UseCase для Zone.
func New(repo Repo) *UseCase { return &UseCase{repo: repo} }

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
// Валидация/нормализация page_size — здесь (use-case владеет валидацией входа);
// repo получает уже нормализованный pageSize (0 → default, >max → InvalidArgument).
func (u *UseCase) List(ctx context.Context, p Pagination) ([]*domain.Zone, string, error) {
	size, err := validate.PageSize("page_size", p.PageSize)
	if err != nil {
		return nil, "", err
	}
	p.PageSize = size
	return u.repo.List(ctx, p)
}

// Create вставляет Zone (admin-only). id/region_id назначаются админом; region_id
// обязан ссылаться на существующий Region (это гарантирует DB FK → FailedPrecondition).
func (u *UseCase) Create(ctx context.Context, id, regionID, name string, st domain.ZoneStatus) (*domain.Zone, error) {
	z := domain.Zone{ID: id, RegionID: regionID, Name: name, Status: st}
	if err := z.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", geoerrors.ErrInvalidArg, err.Error())
	}
	created, err := u.repo.Insert(ctx, &z)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// Update меняет Zone (admin-only). id неизменяем. Атомарно через repo.Update
// (single-statement UPDATE … RETURNING, без Get-then-Update / TOCTOU).
// Партиал-семантика: пустые regionID/name и unspecified-статус НЕ меняют поле
// (передаем nil; repo делает COALESCE-апдейт). Перед записью валидируются только
// заданные новые значения.
func (u *UseCase) Update(ctx context.Context, id, regionID, name string, st domain.ZoneStatus) (*domain.Zone, error) {
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
	// unspecified-статус (нулевое значение) не затирает текущий — меняем только при
	// явно заданном; заданный — валидируем (out-of-range → ErrInvalidArg).
	if st != domain.ZoneStatusUnspecified {
		if err := st.Validate(); err != nil {
			return nil, fmt.Errorf("%w: %s", geoerrors.ErrInvalidArg, err.Error())
		}
		p.Status = &st
	}
	updated, err := u.repo.Update(ctx, id, p)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Delete удаляет Zone (admin-only).
func (u *UseCase) Delete(ctx context.Context, id string) error {
	if id == "" {
		return geoerrors.ErrInvalidArg
	}
	if err := u.repo.Delete(ctx, id); err != nil {
		return err
	}
	return nil
}
