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

// flags variables
var verbosity int
var debug bool

var root = &cobra.Command{
	Use:   "histoctl",
	Short: "Rewrite git history dates",
	// Use persistentPreRun for initializing debug flags and logging through glog.
	PersistentPreRun: func(cmd *cobra.Command, args []string) {

		// translate debug into a verbosity level
		if verbosity != 0 {
			debug = verbosity >= 5
		} else if debug {
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
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var repo *git.Repository
		var commits []histo.Commits
		var historiography *histo.Historiography

		cmd.SilenceUsage = true // do not show usage if an error is returned

		// init processor
		processor := &histo.Processor{
			closedDays, startHour, endHour, make(map[git.Oid]time.Time),
		}

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
			if glog.V(5) {
				glog.Infof("%q", commits) // display all commits retrieved in debug mode
			}
			// if no commits, no need to go further
			if len(commits) == 0 {
				return
			}

			// init historiography struct
			if historiography, err = histo.NewHistoriography(repo, processor); err != nil {
				return
			}
			// be sure to free resources when ending
			defer historiography.Free()

			// infers changes needed to be in sync with the distribution strategy
			if err = historiography.Preprocess(commits); err != nil {
				return
			}

			// logs changes in a convenient if verbosity is hight enough
			if glog.V(2) {
				logs(commits, processor)
			}

			// apply changes on the temporary branch
			if err = historiography.Process(histo.Flatten(commits)); err != nil {
				return
			}

			// ask for confirmation if needed and override branch
			if err = confirm(historiography, repo); err != nil {
				return
			}
		}
		return
	},
}

// Wrapper for the confirmation, call override from historiography object if
// user validate changes, or directly override if force flag has been passed.
func confirm(h *histo.Historiography, repo *git.Repository) (err error) {
	ok := viper.GetBool("Force")
	if !ok {
		ok, err = histo.Confirm(repo)
	}
	if ok {
		err = h.Override()
	}
	return
}

// Logs commits and changes through glog in a readable way.
func logs(commits []histo.Commits, p *histo.Processor) {
	fmt := func(t time.Time) string { return t.Format("15:06") } // all times formatted the same way

	for _, day := range commits {
		// there can't be empty day, at least one commit will exists
		d := day[0].Author().When
		glog.Infof("computing day: %s", d.Format("Mon 02 Jan 2006"))

		for _, commit := range day {
			commitTime := commit.Author().When
			id := commit.Id().String()[:10]
			if changeTime, ok := p.Changes[*commit.Id()]; ok {
				glog.Infof("commit %s at %s changed to %s", id, fmt(commitTime), fmt(changeTime))
			} else {
				glog.Infof("commit %s at %s not changed", id, fmt(commitTime))
			}
		}
	}
}

// Init the root command of histoctl with flags and bind them to viper or internal variables.
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
