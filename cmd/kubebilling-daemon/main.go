package main

import (
	"flag"
	"fmt"
	"github.com/dashjay/kubebilling/pkg/collector"
	"strings"
	"time"
)

var (
	runtimeEndpoint = flag.String("runtime-endpoint", "/run/containerd/containerd.sock", "containerd sock")
	filterLabel     = flag.String("filter-label", "", "label for filtering pods, eg: a=b,c=d")
)

func main() {
	filter, err := parseLabelStringSlice(strings.Split(*filterLabel, ","))
	if err != nil {
		panic(err)
	}
	daemon := collector.NewDaemon(*runtimeEndpoint, "./test.db", 1*time.Minute, filter)
	daemon.MainLoop()
}

func parseLabelStringSlice(ss []string) (map[string]string, error) {
	labels := make(map[string]string)
	for _, s := range ss {
		pair := strings.Split(s, "=")
		if len(pair) != 2 {
			return nil, fmt.Errorf("incorrectly specified label: %v", s)
		}
		labels[pair[0]] = pair[1]
	}
	return labels, nil
}
