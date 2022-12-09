package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel/trace"
	internalapi "k8s.io/cri-api/pkg/apis"
	"k8s.io/kubernetes/pkg/kubelet/cri/remote"

	"github.com/kubernetes-sigs/cri-tools/pkg/common"
	"kubefee/pkg/crictl"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"
)

var (
	// RuntimeEndpoint is CRI server runtime endpoint
	RuntimeEndpoint string
	// RuntimeEndpointIsSet is true when RuntimeEndpoint is configured
	RuntimeEndpointIsSet bool
	// ImageEndpoint is CRI server image endpoint, default same as runtime endpoint
	ImageEndpoint string
	// ImageEndpointIsSet is true when ImageEndpoint is configured
	ImageEndpointIsSet bool
	// Timeout  of connecting to server (default: 10s)
	Timeout time.Duration
	// Debug enable debug output
	Debug bool
	// PullImageOnCreate enables pulling image on create requests
	PullImageOnCreate bool
	// DisablePullOnRun disable pulling image on run requests
	DisablePullOnRun bool
)

const (
	defaultConfigPath = "/etc/crictl.yaml"
)

const defaultTimeout = 2 * time.Second

var defaultRuntimeEndpoints = []string{"unix:///var/run/dockershim.sock", "unix:///run/containerd/containerd.sock", "unix:///run/crio/crio.sock", "unix:///var/run/cri-dockerd.sock"}

var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

func getTimeout(timeDuration time.Duration) time.Duration {
	if timeDuration.Seconds() > 0 {
		return timeDuration
	}
	return defaultTimeout // use default
}

func main() {
	app := cli.NewApp()
	app.Commands = []*cli.Command{listPodCommand}
	runtimeEndpointUsage := fmt.Sprintf("Endpoint of CRI container runtime "+
		"service (default: uses in order the first successful one of %v). "+
		"Default is now deprecated and the endpoint should be set instead.",
		defaultRuntimeEndpoints)
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			EnvVars: []string{"CRI_CONFIG_FILE"},
			Value:   defaultConfigPath,
			Usage:   "Location of the client config file. If not specified and the default does not exist, the program's directory is searched as well",
		},
		&cli.StringFlag{
			Name:    "runtime-endpoint",
			Aliases: []string{"r"},
			EnvVars: []string{"CONTAINER_RUNTIME_ENDPOINT"},
			Usage:   runtimeEndpointUsage,
		},
		&cli.StringFlag{
			Name:    "image-endpoint",
			Aliases: []string{"i"},
			EnvVars: []string{"IMAGE_SERVICE_ENDPOINT"},
			Usage: "Endpoint of CRI image manager service (default: uses " +
				"'runtime-endpoint' setting)",
		},
		&cli.DurationFlag{
			Name:    "timeout",
			Aliases: []string{"t"},
			Value:   defaultTimeout,
			Usage: "Timeout of connecting to the server in seconds (e.g. 2s, 20s.). " +
				"0 or less is set to default",
		},
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"D"},
			Usage:   "Enable debug mode",
		},
	}
	app.Before = func(context *cli.Context) (err error) {
		var config *common.ServerConfiguration
		var exePath string

		if exePath, err = os.Executable(); err != nil {
			logrus.Fatal(err)
		}
		if config, err = common.GetServerConfigFromFile(context.String("config"), exePath); err != nil {
			if context.IsSet("config") {
				logrus.Fatal(err)
			}
		}

		if config == nil {
			RuntimeEndpoint = context.String("runtime-endpoint")
			if context.IsSet("runtime-endpoint") {
				RuntimeEndpointIsSet = true
			}
			ImageEndpoint = context.String("image-endpoint")
			if context.IsSet("image-endpoint") {
				ImageEndpointIsSet = true
			}
			if context.IsSet("timeout") {
				Timeout = getTimeout(context.Duration("timeout"))
			} else {
				Timeout = context.Duration("timeout")
			}
			Debug = context.Bool("debug")
			DisablePullOnRun = false
		} else {
			// Command line flags overrides config file.
			if context.IsSet("runtime-endpoint") {
				RuntimeEndpoint = context.String("runtime-endpoint")
				RuntimeEndpointIsSet = true
			} else if config.RuntimeEndpoint != "" {
				RuntimeEndpoint = config.RuntimeEndpoint
				RuntimeEndpointIsSet = true
			} else {
				RuntimeEndpoint = context.String("runtime-endpoint")
			}
			if context.IsSet("image-endpoint") {
				ImageEndpoint = context.String("image-endpoint")
				ImageEndpointIsSet = true
			} else if config.ImageEndpoint != "" {
				ImageEndpoint = config.ImageEndpoint
				ImageEndpointIsSet = true
			} else {
				ImageEndpoint = context.String("image-endpoint")
			}
			if context.IsSet("timeout") {
				Timeout = getTimeout(context.Duration("timeout"))
			} else if config.Timeout > 0 { // 0/neg value set to default timeout
				Timeout = config.Timeout
			} else {
				Timeout = context.Duration("timeout")
			}
			if context.IsSet("debug") {
				Debug = context.Bool("debug")
			} else {
				Debug = config.Debug
			}
			PullImageOnCreate = config.PullImageOnCreate
			DisablePullOnRun = config.DisablePullOnRun
		}

		if Debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	// sort all flags
	for _, cmd := range app.Commands {
		sort.Sort(cli.FlagsByName(cmd.Flags))
	}
	sort.Sort(cli.FlagsByName(app.Flags))

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

var listPodCommand = &cli.Command{
	Name:                   "pods",
	Usage:                  "List pods",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "filter by pod id",
		},
		&cli.StringFlag{
			Name:  "name",
			Value: "",
			Usage: "filter by pod name regular expression pattern",
		},
		&cli.StringFlag{
			Name:  "namespace",
			Value: "",
			Usage: "filter by pod namespace regular expression pattern",
		},
		&cli.StringFlag{
			Name:    "state",
			Aliases: []string{"s"},
			Value:   "",
			Usage:   "filter by pod state",
		},
		&cli.StringSliceFlag{
			Name:  "label",
			Usage: "filter by key=value label",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "show verbose info for pods",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "list only pod IDs",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output format, One of: json|yaml|table",
			Value:   "table",
		},
		&cli.BoolFlag{
			Name:    "latest",
			Aliases: []string{"l"},
			Usage:   "Show the most recently created pod",
		},
		&cli.IntFlag{
			Name:    "last",
			Aliases: []string{"n"},
			Usage:   "Show last n recently created pods. Set 0 for unlimited",
		},
		&cli.BoolFlag{
			Name:  "no-trunc",
			Usage: "Show output without truncating the ID",
		},
	},
	Action: func(context *cli.Context) error {
		var err error
		runtimeClient, err := getRuntimeService(context, 0)
		if err != nil {
			return err
		}

		opts := crictl.ListOptions{
			Id:                 context.String("id"),
			State:              context.String("state"),
			Verbose:            context.Bool("verbose"),
			Quiet:              context.Bool("quiet"),
			Output:             context.String("output"),
			Latest:             context.Bool("latest"),
			Last:               context.Int("last"),
			NoTrunc:            context.Bool("no-trunc"),
			NameRegexp:         context.String("name"),
			PodNamespaceRegexp: context.String("namespace"),
		}
		opts.Labels, err = parseLabelStringSlice(context.StringSlice("label"))
		if err != nil {
			return err
		}

		tick := time.NewTicker(time.Second)
		for range tick.C {
			psbs, err := crictl.ListPodSandboxes(runtimeClient, opts)
			if err != nil {
				return err
			}
			for _, psb := range psbs {
				logger := logrus.WithFields(logrus.Fields{"name": psb.Metadata.Name, "namespace": psb.Metadata.Namespace, "memory": psb})
				logger.Infoln("pod found")
				psbsStats, err := crictl.ListPodSandboxStats(runtimeClient, psb.Id)
				if err != nil {
					logger.WithError(err).Errorln("list pod sandbox error")
					continue
				}
				if len(psbsStats) != 1 {
					logger.Warnf("len(psbsStatus) == %d", len(psbsStats))
					continue
				}
				psbStats := psbsStats[0]
				if psbStats != nil && psbStats.Linux != nil {
					logger.WithField("cpu", psbStats.Linux.Cpu.String()).WithField("memory", psbStats.Linux.Memory.String()).Infoln("resource")
				}
			}
		}
		return nil
	},
}

func getRuntimeService(context *cli.Context, timeout time.Duration) (res internalapi.RuntimeService, err error) {
	if RuntimeEndpointIsSet && RuntimeEndpoint == "" {
		return nil, fmt.Errorf("--runtime-endpoint is not set")
	}
	logrus.Debug("get runtime connection")

	// Check if a custom timeout is provided.
	t := Timeout
	if timeout != 0 {
		t = timeout
	}

	tp := trace.NewNoopTracerProvider()

	// If no EP set then use the default endpoint types
	if !RuntimeEndpointIsSet {
		logrus.Warningf("runtime connect using default endpoints: %v. "+
			"As the default settings are now deprecated, you should set the "+
			"endpoint instead.", defaultRuntimeEndpoints)
		logrus.Debug("Note that performance maybe affected as each default " +
			"connection attempt takes n-seconds to complete before timing out " +
			"and going to the next in sequence.")

		for _, endPoint := range defaultRuntimeEndpoints {
			logrus.Debugf("Connect using endpoint %q with %q timeout", endPoint, t)

			res, err = remote.NewRemoteRuntimeService(endPoint, t, tp)
			if err != nil {
				logrus.Error(err)
				continue
			}

			logrus.Debugf("Connected successfully using endpoint: %s", endPoint)
			break
		}
		return res, err
	}
	return remote.NewRemoteRuntimeService(RuntimeEndpoint, t, tp)
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
