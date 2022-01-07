// Copyright 2022 PingCAP, Inc. Licensed under Apache-2.0.

package recordandreplay

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/pingcap/tidb-dashboard/pkg/apiserver/model"
	"github.com/pingcap/tidb-dashboard/pkg/dbstore"
	"github.com/pingcap/tidb-dashboard/pkg/utils/topology"
)

type TaskState int

const (
	TaskStateRecording TaskState = iota
	TaskStateFinishRecording
	TaskStateReplaying
	TaskStateFinishReplaying
	TaskStateError
)

type InstanceList []model.RequestTargetNode

func (l *InstanceList) Scan(src interface{}) error {
	return json.Unmarshal([]byte(src.(string)), l)
}

func (l InstanceList) Value() (driver.Value, error) {
	val, err := json.Marshal(l)
	return string(val), err
}

type Task struct {
	ID        string       `gorm:"primary_key;size:40" json:"id"`
	Name      string       `json:"name" gorm:"type:text"`
	Target    InstanceList `json:"target"`
	State     TaskState    `json:"state" gorm:"index"`
	StartTime time.Time    `json:"start_time"`
	EndTime   time.Time    `json:"end_time"`
}

func (Task) TableName() string {
	return "record_and_replay_tasks"
}

func NewTask(name string, startTime time.Time, instances []topology.TiDBInfo, state TaskState) Task {
	target := make([]model.RequestTargetNode, len(instances))
	for i := range instances {
		target[i] = model.RequestTargetNode{
			Kind: model.NodeKindTiDB,
			IP:   instances[i].IP,
			Port: int(instances[i].Port),
		}
	}

	task := Task{
		ID:        uuid.New().String(),
		Name:      name,
		StartTime: startTime,
		Target:    target,
		State:     state,
	}

	return task
}

func GetTasks(db *dbstore.DB) ([]Task, error) {
	var tasks []Task
	err := db.
		Select("id, name, start_time, end_time, state").
		Find(&tasks).Error
	return tasks, err
}

func GetTask(db *dbstore.DB, taskID string) (*Task, error) {
	var task Task
	err := db.Where("id = ?", taskID).First(&task).Error
	return &task, err
}

func UpdateTaskState(db *dbstore.DB, taskID string, state TaskState) error {
	var task Task
	task.ID = taskID
	return db.Model(&task).Update("state", state).Error
}

func UpdateTaskEndtime(db *dbstore.DB, taskID string, endtime int64) error {
	var task Task
	task.ID = taskID
	return db.Model(&task).Update("end_time", time.Unix(endtime, 0)).Error
}

func autoMigrate(db *dbstore.DB) error {
	return db.AutoMigrate(&Task{})
}
