package main

import (
	"flag"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"os"
	"strconv"
	"strings"
)

var (
	force     bool
	debug     bool
	verbosity int
	commits   int
	author    string
	email     string
)

var root = &cobra.Command{
	Use:   "histoctl",
	Short: "Rewrite git history dates",
	// Use persistentPreRun for initializing debug flags and logging through glog.
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if debug {
			verbosity = 5
		}

		// remove args from command line in order to avoid collision with glog
		os.Args = os.Args[:1]

		// re-set flags to pass down config to glog as flag.Parse() is not called
		// by the cobra library
		flag.Set("v", strconv.Itoa(verbosity))
		flag.Set("logtostderr", "true") // for the moment we log to stderr
		flag.Parse()

		// provide informations to the user so there's an indication of the level of verbosity
		if verbosity < 5 {
			glog.V(1).Infof("verbosity: %s", strings.Repeat("v", verbosity))
		} else {
			glog.V(1).Infof("verbosity: debug (vvvvv)")
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true // do not show usage if an error is returned
		return run(args, commits, author, email)
	},
}

// Init the root command of histoctl with flags and bind them to viper or internal variables.
func init() {
	root.PersistentFlags().BoolVarP(&force, "force", "f", false,
		"force change, no review of rescheduling")
	root.PersistentFlags().BoolVar(&debug, "debug", false, "debug mode")
	root.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "verbose")
	root.PersistentFlags().IntVarP(&commits, "commits", "c", -1,
		"number of commits to take into account when rescheduling\n (nth latest)")
	root.PersistentFlags().StringVar(&author, "author", "",
		"replace author by new one on all commits.")
	root.PersistentFlags().StringVar(&email, "email", "",
		"replace email by new one on all commits.")

}

func main() {
	pred := func(err error) bool {
		return (err != nil && bool(glog.V(1)) &&
			!strings.Contains(err.Error(), "unknown flag:"))
	}
	if err := root.Execute(); pred(err) {
		glog.Errorf("%s", err)
	}
}
