package metrics

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joomcode/errorx"
	"github.com/pingcap/log"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/apiserver/user"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/apiserver/utils"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/dbstore"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/httpc"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/utils/topology"
)

var (
	ErrNS                    = errorx.NewNamespace("error.api.metrics")
	ErrPrometheusNotFound    = ErrNS.NewType("prometheus_not_found")
	ErrPrometheusQueryFailed = ErrNS.NewType("prometheus_query_failed")
)

const (
	defaultPromQueryTimeout = time.Second * 30
)

type ServiceParams struct {
	fx.In
	HTTPClient *httpc.Client
	EtcdClient *clientv3.Client
	LocalStore *dbstore.DB
}

type Service struct {
	params       ServiceParams
	lifecycleCtx context.Context
}

func NewService(lc fx.Lifecycle, p ServiceParams) *Service {
	s := &Service{params: p}

	if err := autoMigrate(p.LocalStore); err != nil {
		log.Fatal("Failed to initialize database", zap.Error(err))
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			s.lifecycleCtx = ctx
			return nil
		},
	})

	return s
}

func Register(r *gin.RouterGroup, auth *user.AuthService, s *Service) {
	endpoint := r.Group("/metrics")
	endpoint.POST("/alert_webhook", s.alertManagerWebHook)
	endpoint.Use(auth.MWAuthRequired())
	endpoint.GET("/query", s.queryHandler)
	endpoint.GET("/alerts", s.alertsHandler)
	endpoint.GET("/alert_channels/list", s.listAlertChannels)
	endpoint.PUT("/alert_channels/create", s.createAlertChannel)
	endpoint.GET("/alert_channels/get/:id", s.getAlertChannel)
	endpoint.POST("/alert_channels/update", s.updateAlertChannel)
	endpoint.DELETE("/alert_channels/delete/:id", s.deleteAlertChannel)
}

type QueryRequest struct {
	StartTimeSec int    `json:"start_time_sec" form:"start_time_sec"`
	EndTimeSec   int    `json:"end_time_sec" form:"end_time_sec"`
	StepSec      int    `json:"step_sec" form:"step_sec"`
	Query        string `json:"query" form:"query"`
}

type QueryResponse struct {
	Status string                 `json:"status"`
	Data   map[string]interface{} `json:"data"`
}

// @ID metricsQuery
// @Summary Query metrics
// @Description Query metrics in the given range
// @Param q query QueryRequest true "Query"
// @Success 200 {object} QueryResponse
// @Failure 401 {object} utils.APIError "Unauthorized failure"
// @Security JwtAuth
// @Router /metrics/query [get]
func (s *Service) queryHandler(c *gin.Context) {
	var req QueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.MakeInvalidRequestErrorFromError(c, err)
		return
	}

	pi, err := topology.FetchPrometheusTopology(s.lifecycleCtx, s.params.EtcdClient)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if pi == nil {
		_ = c.Error(ErrPrometheusNotFound.NewWithNoMessage())
		return
	}

	params := url.Values{}
	params.Add("query", req.Query)
	params.Add("start", strconv.Itoa(req.StartTimeSec))
	params.Add("end", strconv.Itoa(req.EndTimeSec))
	params.Add("step", strconv.Itoa(req.StepSec))

	uri := fmt.Sprintf("http://%s:%d/api/v1/query_range?%s", pi.IP, pi.Port, params.Encode())
	promReq, err := http.NewRequestWithContext(s.lifecycleCtx, http.MethodGet, uri, nil)
	if err != nil {
		_ = c.Error(ErrPrometheusQueryFailed.Wrap(err, "failed to build Prometheus request"))
		return
	}

	promResp, err := s.params.HTTPClient.WithTimeout(defaultPromQueryTimeout).Do(promReq)
	if err != nil {
		_ = c.Error(ErrPrometheusQueryFailed.Wrap(err, "failed to send requests to Prometheus"))
		return
	}

	defer promResp.Body.Close()
	if promResp.StatusCode != http.StatusOK {
		_ = c.Error(ErrPrometheusQueryFailed.New("failed to query Prometheus"))
		return
	}

	body, err := ioutil.ReadAll(promResp.Body)
	if err != nil {
		_ = c.Error(ErrPrometheusQueryFailed.Wrap(err, "failed to read Prometheus query result"))
		return
	}

	c.Data(promResp.StatusCode, promResp.Header.Get("content-type"), body)
}

// @ID metricsGetAlerts
// @Summary Get alerts
// @Description Get alerts
// @Produce json
// @Success 200 {object} QueryResponse
// @Failure 401 {object} utils.APIError "Unauthorized failure"
// @Security JwtAuth
// @Router /metrics/alerts [get]
func (s *Service) alertsHandler(c *gin.Context) {
	pi, err := topology.FetchPrometheusTopology(s.lifecycleCtx, s.params.EtcdClient)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if pi == nil {
		_ = c.Error(ErrPrometheusNotFound.NewWithNoMessage())
		return
	}

	// FIXME: Reduce duplication

	uri := fmt.Sprintf("http://%s:%d/api/v1/rules", pi.IP, pi.Port)
	promReq, err := http.NewRequestWithContext(s.lifecycleCtx, http.MethodGet, uri, nil)
	if err != nil {
		_ = c.Error(ErrPrometheusQueryFailed.Wrap(err, "failed to build Prometheus request"))
		return
	}

	promResp, err := s.params.HTTPClient.WithTimeout(defaultPromQueryTimeout).Do(promReq)
	if err != nil {
		_ = c.Error(ErrPrometheusQueryFailed.Wrap(err, "failed to send requests to Prometheus"))
		return
	}

	defer promResp.Body.Close()
	if promResp.StatusCode != http.StatusOK {
		_ = c.Error(ErrPrometheusQueryFailed.New("failed to query Prometheus"))
		return
	}

	body, err := ioutil.ReadAll(promResp.Body)
	if err != nil {
		_ = c.Error(ErrPrometheusQueryFailed.Wrap(err, "failed to read Prometheus query result"))
		return
	}

	c.Data(promResp.StatusCode, promResp.Header.Get("content-type"), body)
}
