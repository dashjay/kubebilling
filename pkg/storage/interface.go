package storage

import "github.com/dashjay/kubebilling/pkg/records"

type Interface interface {
	AddPodUsageRecord(pur *records.PodUsageRecord) error
	AddPodBaseRecord(pbr *records.PodBaseRecord) error
}
