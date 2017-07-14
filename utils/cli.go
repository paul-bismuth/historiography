package utils

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var verbosity int
var debug bool

func defaultPersistentPreRun(cmd *cobra.Command, args []string) {
	if verbosity != 0 {
		debug = verbosity >= 5
	} else if debug {
		verbosity = 5
	}
	SetupGlog(verbosity)
}

func InitCli(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolP("force", "f", false, "force change, no review of rescheduling")
	cmd.PersistentFlags().BoolVar(&debug, "debug", false, "debug mode")
	cmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "verbose")
	cmd.PersistentPreRun = defaultPersistentPreRun

	viper.BindPFlag("Force", cmd.PersistentFlags().Lookup("force"))

	viper.SetConfigName("historiography")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HISTORIOGRAPHY_CONFIG" + string(os.PathSeparator))
}
