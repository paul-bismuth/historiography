package main

import (
	"github.com/golang/glog"
	"github.com/paul-bismuth/historiography/utils"
	"github.com/spf13/cobra"
	git "gopkg.in/libgit2/git2go.v25"
	"time"
)

const format = "20060102"

var root = &cobra.Command{
	Use:   "historiography",
	Short: "Rewrite git history dates",
	Run: func(cmd *cobra.Command, args []string) {
		for _, arg := range args {
			if repo, err := git.OpenRepository(arg); err != nil {
				utils.MustG(err)
			} else {
				defer repo.Free()
				WalkRepo(repo)
			}
		}
	},
}

func reorganise(commits [][]*git.Commit) map[*git.Oid]time.Time {
	commitsMap := make(map[*git.Oid]time.Time)
	if glog.V(2) {
		glog.Infof("%q", commits)
	}
	for _, commitsArray := range commits {
		commit := commitsArray[0]
		if day := commit.Author().When.Weekday(); day == 0 || day == 6 {
			continue
		}

		if glog.V(1) {
			glog.Infof("computed day: %s", commit.Author().When.Format("Mon 02 Jan 2006"))
		}
	}

	return commitsMap
}

func WalkRepo(repo *git.Repository) {
	day, year, month := 0, 0, time.January
	commits := [][]*git.Commit{}

	if rev, err := repo.Walk(); err != nil {
		utils.MustG(err)
	} else {
		defer rev.Free()
		rev.Sorting(git.SortTime)
		utils.MustG(rev.PushHead())
		glog.Infof("parsing %s repository", repo.Workdir())
		utils.MustG(rev.Iterate(func(commit *git.Commit) bool {
			date := commit.Author().When
			if day != date.Day() || month != date.Month() || year != date.Year() {
				commits = append(commits, []*git.Commit{})
				year, month, day = date.Date()
			}
			commits[len(commits)-1] = append(commits[len(commits)-1], commit)

			return true
		}))
		reorganise(commits)
	}
}

func init() {
	utils.InitCli(root)
}

func main() {
	utils.Must(root.Execute())
}
