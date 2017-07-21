package main

import (
	"flag"
	histo "github.com/backinmydays/historiography"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	git "gopkg.in/libgit2/git2go.v26"
	"os"
	"strconv"
	"strings"
	"time"
)

// opening hours 9h - 18h -> repartition goes from 8h - 9h, 18h - 02h
const startHour = 9
const endHour = 18

var closedDays = []time.Weekday{time.Saturday, time.Sunday}
var verbosity int
var debug bool

var root = &cobra.Command{
	Use:   "histoctl",
	Short: "Rewrite git history dates",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if verbosity != 0 {
			debug = verbosity >= 5
		} else if debug {
			verbosity = 5
		}
		os.Args = os.Args[:1]

		flag.Set("v", strconv.Itoa(verbosity))
		flag.Set("logtostderr", "true")
		flag.Parse()
		if verbosity < 5 {
			glog.V(1).Infof("verbosity: %s", strings.Repeat("v", verbosity))
		} else {
			glog.V(1).Infof("verbosity: debug (vvvvv)")
		}
	},
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var repo *git.Repository
		var commits []histo.Commits
		var changes histo.Changes
		var historiography *histo.Historiography

		cmd.SilenceUsage = true // do not show usage if an error is returned

		// init distribute
		distribute := &histo.Distribute{startHour, endHour, closedDays}

		for _, arg := range args {
			if repo, err = git.OpenRepository(arg); err != nil {
				return
			}

			if glog.V(1) {
				glog.Infof("parsing %s repository", repo.Workdir())
			}

			defer repo.Free()
			// retrieve all commits from HEAD
			if commits, err = histo.Retrieve(repo); err != nil {
				return
			}
			// if no commits, no need to go further
			if len(commits) == 0 {
				return
			}
			// init historiography struct
			if historiography, err = histo.NewHistoriography(repo); err != nil {
				return
			}
			defer historiography.Free()

			changes = histo.Reorganise(commits, distribute)

			if glog.V(2) {
				logs(histo.Flatten(commits), changes)
			}

			if err = historiography.Process(histo.Flatten(commits), changes); err != nil {
				return
			}
			if err = historiography.Confirm(viper.GetBool("Force")); err != nil {
				return
			}
		}
		return
	},
}

func logs(commits histo.Commits, changes histo.Changes) {
}

func init() {
	root.PersistentFlags().BoolP("force", "f", false, "force change, no review of rescheduling")
	root.PersistentFlags().BoolVar(&debug, "debug", false, "debug mode")
	root.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "verbose")

	viper.BindPFlag("Force", root.PersistentFlags().Lookup("force"))
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
