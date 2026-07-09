// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"errors"
	"net"
	"testing"

	"google.golang.org/grpc"
)

// fakeServeServer — тестовый двойник gracefulServer: Serve сразу возвращает
// заданную ошибку (без реального listen'а).
type fakeServeServer struct{ err error }

func (f fakeServeServer) Serve(net.Listener) error { return f.err }

// TestRunInternalListener_fatalError_cancelsRootCtx — фатальная (не graceful
// ErrServerStopped) ошибка internalSrv.Serve обязана снести весь процесс через
// cancel() root-ctx, а не только залогироваться: иначе admin-плоскость :9091
// (InternalRegion/ZoneService) молча ложится, процесс остаётся «здоровым» на
// public :9090, оркестратор не рестартит (нет non-zero exit). Наблюдаемое
// поведение — root-ctx отменён (симметрия с public-листенером).
func TestRunInternalListener_fatalError_cancelsRootCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runInternalListener(fakeServeServer{err: errors.New("fatal accept error")}, nil, cancel, quietLogger())

	select {
	case <-ctx.Done():
		// ожидаемо: fatal Serve-ошибка отменила root-ctx.
	default:
		t.Fatal("root ctx not cancelled after fatal internal Serve error — admin plane would be silently down")
	}
}

// TestRunInternalListener_gracefulStop_doesNotCancel — штатный graceful shutdown
// (Serve → grpc.ErrServerStopped после GracefulStop) НЕ должен трактоваться как
// фатальный: cancel() не вызывается (иначе двойной teardown/гонка на штатном пути).
func TestRunInternalListener_gracefulStop_doesNotCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runInternalListener(fakeServeServer{err: grpc.ErrServerStopped}, nil, cancel, quietLogger())

	select {
	case <-ctx.Done():
		t.Fatal("root ctx cancelled on graceful ErrServerStopped — should be treated as normal shutdown")
	default:
		// ожидаемо: graceful-stop не отменяет root-ctx.
	}
}
