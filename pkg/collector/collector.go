package collector

import (
	"context"
	"github.com/dashjay/kubebilling/pkg/crictl"
	"github.com/dashjay/kubebilling/pkg/records"
	"github.com/dashjay/kubebilling/pkg/storage"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimePb "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/kubernetes/pkg/kubelet/cri/remote"
	"time"
)

type Daemon struct {
	Storage storage.Interface
	internalapi.RuntimeService

	// filter conditions
	filterLabels map[string]string

	loopTime time.Duration
}

func NewDaemon(runtimeEndpoint string, storageConfig string, loopTime time.Duration, filter map[string]string) *Daemon {
	cli, err := getRuntimeService(runtimeEndpoint, time.Second*3)
	if err != nil {
		panic(err)
	}
	sto, err := storage.NewSqlite(storageConfig)
	if err != nil {
		panic(err)
	}
	return &Daemon{
		Storage:        sto,
		RuntimeService: cli,
		loopTime:       loopTime,
		filterLabels:   filter,
	}
}

func (d *Daemon) MainLoop() {
	tick := time.NewTicker(d.loopTime)
	for range tick.C {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), d.loopTime)
			defer cancel()
			start := time.Now().UTC().Unix()
			logrus.WithField("time", start).Infoln("MainLoop Running")
			defer logrus.WithField("time-consume", time.Now().Unix()-start).Infoln("MainLoop finished")
			psbs, err := d.GetSandBoxesSnapshot()
			if err != nil {
				logrus.WithError(err).Errorln("GetSandBoxesSnapshot error")
				return
			}
			d.HandleSandBoxes(ctx, psbs)
		}()
	}
}

func (d *Daemon) HandleSandBoxes(ctx context.Context, in []*runtimePb.PodSandbox) {
	logrus.WithField("count", len(in)).Debugln("HandleSandBoxes")
	for _, psb := range in {
		logger := logrus.WithFields(logrus.Fields{"name": psb.Metadata.Name, "namespace": psb.Metadata.Namespace})
		logger.Infoln("pod found, try collect statistics on usage")
		psbStatus, err := crictl.GetPodSandboxStats(d.RuntimeService, psb.Id)
		if err != nil {
			logger.WithError(err).Errorln("GetPodSandboxStats error")
			continue
		}
		//bin, _ := json.Marshal(gojsonq.New().FromString(psbStatus.Info["info"]).Find("config.linux.resources"))
		//logger.Infof("sbStatus.Info = %s", bin)
		//bin, _ = json.Marshal(psbStatus.Status)
		//logger.Infof("psbStatus.Status = %s", bin)

		now := time.Now().UTC().Unix()
		rec := &records.PodUsageRecord{
			CreatedAt: now,
			UID:       psbStatus.Status.Metadata.Uid,
			State:     psbStatus.Status.State,
		}
		pbr := records.PodBaseRecord{
			UID:       psbStatus.Status.Metadata.Uid,
			CreatedAt: now,
		}
		err = d.Storage.AddPodBaseRecord(&pbr)
		if err != nil {
			logger.WithField("uid", pbr.UID).WithError(err).Errorln("create pbr error")
			continue
		}
		err = d.Storage.AddPodUsageRecord(rec)
		if err != nil {
			logger.WithField("uid", rec.UID).WithError(err).Errorln("insert to database error")
			continue
		}
	}
}

func (d *Daemon) GetSandBoxesSnapshot() ([]*runtimePb.PodSandbox, error) {
	opts := crictl.ListOptions{
		Labels: d.filterLabels,
	}
	return crictl.ListPodSandboxes(d.RuntimeService, opts)
}

func getRuntimeService(runtimeEndpoint string, timeout time.Duration) (internalapi.RuntimeService, error) {
	return remote.NewRemoteRuntimeService(runtimeEndpoint, timeout, trace.NewNoopTracerProvider())
}
