// Copyright 2022 PingCAP, Inc. Licensed under Apache-2.0.

package recordandreplay

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pingcap/log"
	"go.uber.org/zap"

	"github.com/pingcap/tidb-dashboard/pkg/apiserver/user"
	"github.com/pingcap/tidb-dashboard/pkg/apiserver/utils"
	"github.com/pingcap/tidb-dashboard/pkg/utils/topology"
	"github.com/pingcap/tidb-dashboard/util/distro"
	"github.com/pingcap/tidb-dashboard/util/rest"
)

const (
	// TODO: use go template for better readability.
	recordURLTemplate = "http://%s:10080/record/%s/%s/%d"
	replayURLTemplate = "http://%s:10080/replay/%s/%s"
)

type TaskRequest struct {
	Name string `json:"name"`
	Time int64  `json:"time"`
}

func RegisterRouter(r *gin.RouterGroup, auth *user.AuthService, s *Service) {
	endpoint := r.Group("/recordandreplay")

	endpoint.Use(auth.MWAuthRequired())
	endpoint.Use(utils.MWForbidByExperimentalFlag(s.params.Config.EnableExperimental))

	endpoint.GET("/tasks", s.GetAllTasks)
	endpoint.GET("/tasks/:id", s.GetTaskByID)
	endpoint.DELETE("/tasks/:id", s.DeleteTask)

	endpoint.POST("/tasks/record", s.StartRecordTask)
	endpoint.GET("/tasks/:id/stop_record", s.StopRecordTask)

	endpoint.GET("/tasks/:id/start_replay", s.StartReplayTask)
	// endpoint.GET("/tasks/:id/stop_replay", s.StopReplayTask)
}

// @Summary Create and start a workload record task
// @Param request body TaskRequest true "Request body"
// @Security JwtAuth
// @Success 200 {object} Task
// @Failure 400 {object} rest.ErrorResponse
// @Failure 401 {object} rest.ErrorResponse
// @Failure 500 {object} rest.ErrorResponse
// @Router /recordandreplay/tasks/record [post]
func (s *Service) StartRecordTask(c *gin.Context) {
	var req TaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(rest.ErrBadRequest.NewWithNoMessage())
		return
	}

	log.Debug("start record task",
		zap.String("name", req.Name),
		zap.Int64("time", req.Time),
	)

	instances, err := topology.FetchTiDBTopology(s.lifecycleCtx, s.params.EtcdClient)
	if err != nil {
		_ = c.Error(err)
		return
	}

	var wg sync.WaitGroup
	cnt := 0
	target := make([]topology.TiDBInfo, 0, len(instances))
	for _, instance := range instances {
		if instance.Status == topology.ComponentStatusUp {
			target = append(target, instance)
			wg.Add(1)
			go func() {
				defer wg.Done()
				uri := fmt.Sprintf(recordURLTemplate, instance.IP, req.Name, "on", req.Time)
				if _, err := s.params.HTTPClient.Send(s.lifecycleCtx, uri, http.MethodGet, nil, ErrHTTPClientRequestFailed, distro.R().TiDB); err != nil {
					_ = c.Error(err)
					log.Warn("failed to start recording on instance",
						zap.String("instance", instance.IP),
						zap.Error(err))
				} else {
					cnt++
				}
			}()
		}
	}

	wg.Wait()
	var task Task
	if cnt > 0 {
		task = NewTask(req.Name, time.Unix(req.Time, 0), target, TaskStateRecording)
	} else {
		task = NewTask(req.Name, time.Unix(req.Time, 0), target, TaskStateError)
	}

	if err := s.params.LocalStore.Create(&task).Error; err != nil {
		_ = c.Error(err)
		log.Error("failed to insert task to db", zap.Error(err))
		return
	}

	c.JSON(http.StatusOK, task)
}

// @Summary List all tasks
// @Security JwtAuth
// @Success 200 {array} Task
// @Failure 401 {object} rest.ErrorResponse
// @Failure 500 {object} rest.ErrorResponse
// @Router /recordandreplay/tasks [get]
func (s *Service) GetAllTasks(c *gin.Context) {
	var tasks []*Task
	err := s.params.LocalStore.Find(&tasks).Error
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, tasks)
}

// @Summary Get a task by ID
// @Security JwtAuth
// @Param id path string true "task ID"
// @Success 200 {object} Task
// @Failure 401 {object} rest.ErrorResponse
// @Failure 500 {object} rest.ErrorResponse
// @Router /recordandreplay/tasks/{id} [get]
func (s *Service) GetTaskByID(c *gin.Context) {
	taskID := c.Param("id")
	log.Debug("get task by id", zap.String("taskID", taskID))

	var task Task
	if err := s.params.LocalStore.Where("id = ?", taskID).Find(&task).Error; err != nil {
		log.Error("can't find task", zap.String("taskID", taskID))
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, task)
}

// @Summary Stop recording
// @Security JwtAuth
// @Param id path string true "task ID"
// @Success 200 {object} rest.EmptyResponse
// @Failure 401 {object} rest.ErrorResponse
// @Failure 500 {object} rest.ErrorResponse
// @Router /recordandreplay/tasks/{id}/stop_record [get]
func (s *Service) StopRecordTask(c *gin.Context) {
	taskID := c.Param("id")
	log.Debug("stop record task", zap.String("taskID", taskID))

	var task Task
	if err := s.params.LocalStore.Where("id = ?", taskID).Find(&task).Error; err != nil {
		log.Error("can't find task", zap.String("taskID", taskID))
		_ = c.Error(err)
		return
	}

	var wg sync.WaitGroup
	endTime := time.Now().Unix()
	for _, instance := range task.Target {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			uri := fmt.Sprintf(recordURLTemplate, ip, task.Name, "off", endTime)
			if _, err := s.params.HTTPClient.Send(s.lifecycleCtx, uri, http.MethodGet, nil, ErrHTTPClientRequestFailed, distro.R().TiDB); err != nil {
				log.Warn("failed to stop recording on instance",
					zap.String("instance", ip),
					zap.Error(err))
			}
		}(instance.IP)
	}

	wg.Wait()
	if err := UpdateTaskEndtime(s.params.LocalStore, taskID, endTime); err != nil {
		e := fmt.Errorf("failed to update task endtime: %v", err)
		_ = c.Error(e)
		return
	}
	if err := UpdateTaskState(s.params.LocalStore, taskID, TaskStateFinishRecording); err != nil {
		e := fmt.Errorf("failed to update task state: %v", err)
		_ = c.Error(e)
		return
	}
	c.JSON(http.StatusOK, rest.EmptyResponse{})
}

// @Summary Start replaying workload
// @Security JwtAuth
// @Param id path string true "task ID"
// @Success 200 {object} rest.EmptyResponse
// @Failure 401 {object} rest.ErrorResponse
// @Failure 500 {object} rest.ErrorResponse
// @Router /recordandreplay/tasks/{id}/start_replay [get]
func (s *Service) StartReplayTask(c *gin.Context) {
	taskID := c.Param("id")
	log.Debug("start replay task", zap.String("taskID", taskID))

	var task Task
	if err := s.params.LocalStore.Where("id = ?", taskID).Find(&task).Error; err != nil {
		_ = c.Error(err)
		return
	}

	var wg sync.WaitGroup
	for _, instance := range task.Target {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			uri := fmt.Sprintf(replayURLTemplate, ip, task.Name, "on")
			if _, err := s.params.HTTPClient.Send(s.lifecycleCtx, uri, http.MethodGet, nil, ErrHTTPClientRequestFailed, distro.R().TiDB); err != nil {
				_ = c.Error(err)
				log.Warn("failed to start replaying on instance",
					zap.String("url", uri),
					zap.Error(err))
			}
			log.Debug("start replay", zap.String("url", uri))
		}(instance.IP)
	}

	if err := UpdateTaskState(s.params.LocalStore, taskID, TaskStateReplaying); err != nil {
		e := fmt.Errorf("failed to update task state: %v", err)
		_ = c.Error(e)
		return
	}

	wg.Wait()
	log.Debug("begin sleep", zap.Duration("time", task.EndTime.Sub(task.StartTime)))
	time.Sleep(task.EndTime.Sub(task.StartTime))
	log.Debug("stop sleep")

	for _, instance := range task.Target {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			uri := fmt.Sprintf(replayURLTemplate, ip, task.Name, "off")
			if _, err := s.params.HTTPClient.Send(s.lifecycleCtx, uri, http.MethodGet, nil, ErrHTTPClientRequestFailed, distro.R().TiDB); err != nil {
				_ = c.Error(err)
				log.Warn("failed to stop replaying on instance",
					zap.String("url", uri),
					zap.Error(err))
			}
		}(instance.IP)
	}
	wg.Wait()

	if err := UpdateTaskState(s.params.LocalStore, taskID, TaskStateFinishReplaying); err != nil {
		log.Error("failed to update task state", zap.Error(err))
	}

	c.JSON(http.StatusOK, rest.EmptyResponse{})
}

// @Summary Delete a task
// @Param id path string true "task ID"
// @Security JwtAuth
// @Success 200 {object} rest.EmptyResponse
// @Failure 401 {object} rest.ErrorResponse
// @Failure 500 {object} rest.ErrorResponse
// @Router /recordandreplay/tasks/{id} [delete]
func (s *Service) DeleteTask(c *gin.Context) {
	taskID := c.Param("id")
	log.Debug("delete task", zap.String("taskID", taskID))

	task := Task{}
	if err := s.params.LocalStore.Where("id = ?", taskID).First(&task).Error; err != nil {
		log.Error("failed to find task", zap.String("taskID", taskID))
		_ = c.Error(err)
		return
	}

	if err := s.params.LocalStore.Where("id = ?", taskID).Delete(&Task{}).Error; err != nil {
		log.Error("failed to delete task", zap.String("taskID", taskID))
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, rest.EmptyResponse{})
}

// func (s *Service) StopReplayTask(c *gin.Context) {
// 	taskID := c.Param("id")
//
// 	var task Task
// 	if err := s.params.LocalStore.Where("id = ?", taskID).Find(&task).Error; err != nil {
// 		_ = c.Error(err)
// 		return
// 	}
//
// 	var wg sync.WaitGroup
// 	for _, instance := range task.Target {
// 		wg.Add(1)
// 		go func(ip string) {
// 			defer wg.Done()
// 			uri := fmt.Sprintf(replayURLTemplate, ip, task.Name, "off")
// 			if _, err := s.params.HTTPClient.Send(s.lifecycleCtx, uri, http.MethodGet, nil, ErrHTTPClientRequestFailed, distro.R().TiDB); err != nil {
// 				_ = c.Error(err)
// 				log.Warn("failed to stop replaying on instance",
// 					zap.String("instance", ip),
// 					zap.Error(err))
// 			}
// 		}(instance.IP)
// 	}
//
// 	wg.Wait()
// 	if err := UpdateTaskState(s.params.LocalStore, taskID, TaskStateAbortReplaying); err != nil {
// 		e := fmt.Errorf("failed to update task state: %v", err)
// 		_ = c.Error(e)
// 		return
// 	}
// 	c.JSON(http.StatusOK, rest.EmptyResponse{})
// }
