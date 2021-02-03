package run

import (
	"carina"
	"carina/utils"
	"flag"
	"fmt"
	"github.com/spf13/cobra"
	"k8s.io/klog"
	"os"
)

var config struct {
	csiSocket   string
	metricsAddr string
	development bool
}

var rootCmd = &cobra.Command{
	Use:     "carina-node",
	Version: carina.Version,
	Short:   "Carina CSI node",
	Long: `carina-node provides CSI node service.
It also works as a custom Kubernetes controller.

The node name where this program runs must be given by either
NODE_NAME environment variable or --nodename flag.`,

	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return subMain()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	fs := rootCmd.Flags()
	fs.StringVar(&config.csiSocket, "csi-socket", utils.DefaultCSISocket, "UNIX domain socket filename for CSI")
	fs.StringVar(&config.metricsAddr, "metrics-addr", ":8080", "Listen address for metrics")
	fs.BoolVar(&config.development, "development", true, "Use development logger config")

	goflags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(goflags)
	fs.AddGoFlagSet(goflags)
}
