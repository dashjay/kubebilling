package records

import (
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type PodUsageRecord struct {
	ID        uint               `gorm:"primarykey"`
	CreatedAt int64              `gorm:"index:created_at"`
	UID       string             `json:"uid" bson:"uid" gorm:"index:uid"`
	State     v1.PodSandboxState `json:"status" bson:"status"`
}

type PodBaseRecord struct {
	UID       string `gorm:"index:uid,unique"`
	CreatedAt int64  `gorm:"index:created_at"`
}
