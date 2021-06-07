package e2e

import (
	"context"
	"flag"
	"github.com/orbs-network/orbs-network-go/bootstrap"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/instrumentation"
	"github.com/orbs-network/orbs-network-go/synchronization/supervised"
	"github.com/orbs-network/orbs-network-go/test/with"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func TestProfile(t *testing.T) {
	if os.Getenv("ENABLE_PROFILE") != "true" {
		t.Skip()
	}

	with.Context(func(ctx context.Context) {
		logger := instrumentation.GetBootstrapCrashLogger()
		var node *bootstrap.Node
		httpAddress := flag.String("listen", ":8080", "ip address and port for http server")
		pathToLog := flag.String("log", "", "path/to/node.log")

		filePaths := config.FilesPaths{
			"./_profile/node1.json",
			"./_profile/node1.keys.json",
		}
		flag.Var(&filePaths, "config", "path/to/config.json")

		flag.Parse()

		cfg, err := config.GetNodeConfigFromFiles(filePaths, *httpAddress)
		require.NoError(t, err)

		logger = instrumentation.GetLogger(*pathToLog, true, cfg)

		node = bootstrap.NewNode(
			cfg,
			logger,
		)

		supervised.NewShutdownListener(logger, node).ListenToOSShutdownSignal()
		<-time.After(2 * time.Minute)

		node.GracefulShutdown(ctx)
	})
}
