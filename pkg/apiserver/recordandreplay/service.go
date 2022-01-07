// Copyright 2022 PingCAP, Inc. Licensed under Apache-2.0.

package recordandreplay

import (
	"context"

	"github.com/joomcode/errorx"
	"github.com/pingcap/log"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/pingcap/tidb-dashboard/pkg/config"
	"github.com/pingcap/tidb-dashboard/pkg/dbstore"
	"github.com/pingcap/tidb-dashboard/pkg/httpc"
	"github.com/pingcap/tidb-dashboard/pkg/pd"
)

var (
	ErrNS                         = errorx.NewNamespace("error.api.record_and_replay")
	ErrHTTPClientRequestFailed    = ErrNS.NewType("http_client_request_failed")
	ErrIgnoredRequest             = ErrNS.NewType("ignored_request")
	ErrTimeout                    = ErrNS.NewType("timeout")
	ErrUnsupportedProfilingType   = ErrNS.NewType("unsupported_profiling_type")
	ErrUnsupportedProfilingTarget = ErrNS.NewType("unsupported_profiling_target")
)

type ServiceParams struct {
	fx.In
	Config     *config.Config
	LocalStore *dbstore.DB

	PDClient   *pd.Client
	EtcdClient *clientv3.Client
	HTTPClient *httpc.Client
}

type Service struct {
	params       ServiceParams
	lifecycleCtx context.Context
}

func NewService(lc fx.Lifecycle, p ServiceParams) *Service {
	if err := autoMigrate(p.LocalStore); err != nil {
		log.Fatal("Failed to initialize database", zap.Error(err))
	}

	service := &Service{params: p}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			service.lifecycleCtx = ctx
			return nil
		},
	})

	return service
}
