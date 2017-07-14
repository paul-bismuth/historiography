package main

import (
	"github.com/golang/glog"
	histo "github.com/paul-bismuth/historiography"
	"github.com/paul-bismuth/historiography/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	git "gopkg.in/libgit2/git2go.v26"
	"strings"
	"time"
)

// opening hours 9h - 18h -> repartition goes from 8h - 9h, 18h - 02h
const startHour = 9
const endHour = 18

var closedDays = []time.Weekday{time.Saturday, time.Sunday}

var root = &cobra.Command{
	Use:   "histoctl",
	Short: "Rewrite git history dates",
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
			defer repo.Free()

			// init historiography struct
			if historiography, err = histo.NewHistoriography(repo); err != nil {
				return
			}
			defer historiography.Free()

			if commits, err = histo.Retrieve(repo); err != nil {
				return
			}
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

func logs(commits histo.Commits, changes histo.Change) {
}

func init() {
	utils.InitCli(root)
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
