package crictl

import (
	"github.com/sirupsen/logrus"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimePb "k8s.io/cri-api/pkg/apis/runtime/v1"
	"log"
	"regexp"
	"sort"
	"strings"
)

type ListOptions struct {
	// Id of container or sandbox
	Id string
	// podID of container
	podID string
	// Regular expression pattern to match pod or container
	NameRegexp string
	// Regular expression pattern to match the pod namespace
	PodNamespaceRegexp string
	// State of the sandbox
	State string
	// show Verbose info for the sandbox
	Verbose bool
	// Labels are selectors for the sandbox
	Labels map[string]string
	// Quiet is for listing just container/sandbox/image IDs
	Quiet bool
	// Output format
	Output string
	// all containers
	all bool
	// Latest container
	Latest bool
	// Last n containers
	Last int
	// out with truncating the Id
	NoTrunc bool
	// image used by the container
	image string
	// resolve image path
	resolveImagePath bool
}

func ListPodSandboxes(client internalapi.RuntimeService, opts ListOptions) ([]*runtimePb.PodSandbox, error) {
	filter := &runtimePb.PodSandboxFilter{}
	if opts.Id != "" {
		filter.Id = opts.Id
	}
	if opts.State != "" {
		st := &runtimePb.PodSandboxStateValue{}
		st.State = runtimePb.PodSandboxState_SANDBOX_NOTREADY
		switch strings.ToLower(opts.State) {
		case "ready":
			st.State = runtimePb.PodSandboxState_SANDBOX_READY
			filter.State = st
		case "notready":
			st.State = runtimePb.PodSandboxState_SANDBOX_NOTREADY
			filter.State = st
		default:
			log.Fatalf("--state should be ready or notready")
		}
	}
	if opts.Labels != nil {
		filter.LabelSelector = opts.Labels
	}
	request := &runtimePb.ListPodSandboxRequest{
		Filter: filter,
	}
	logrus.Debugf("ListPodSandboxRequest: %v", request)
	r, err := client.ListPodSandbox(filter)
	logrus.Debugf("ListPodSandboxResponse: %v", r)
	if err != nil {
		return nil, err
	}
	return getSandboxesList(r, opts), nil
}

func ListPodSandboxStats(client internalapi.RuntimeService, podId string) ([]*runtimePb.PodSandboxStats, error) {
	return client.ListPodSandboxStats(&runtimePb.PodSandboxStatsFilter{Id: podId})
}

type sandboxByCreated []*runtimePb.PodSandbox

func (a sandboxByCreated) Len() int      { return len(a) }
func (a sandboxByCreated) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a sandboxByCreated) Less(i, j int) bool {
	return a[i].CreatedAt > a[j].CreatedAt
}

func matchesRegex(pattern, target string) bool {
	if pattern == "" {
		return true
	}
	matched, err := regexp.MatchString(pattern, target)
	if err != nil {
		// Assume it's not a match if an error occurs.
		return false
	}
	return matched
}

func getSandboxesList(sandboxesList []*runtimePb.PodSandbox, opts ListOptions) []*runtimePb.PodSandbox {
	filtered := []*runtimePb.PodSandbox{}
	for _, p := range sandboxesList {
		// Filter by pod name/namespace regular expressions.
		if matchesRegex(opts.NameRegexp, p.Metadata.Name) &&
			matchesRegex(opts.PodNamespaceRegexp, p.Metadata.Namespace) {
			filtered = append(filtered, p)
		}
	}

	sort.Sort(sandboxByCreated(filtered))
	n := len(filtered)
	if opts.Latest {
		n = 1
	}
	if opts.Last > 0 {
		n = opts.Last
	}
	n = func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}(n, len(filtered))

	return filtered[:n]
}
