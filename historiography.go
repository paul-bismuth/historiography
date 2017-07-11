package main

import (
	"github.com/golang/glog"
	"github.com/paul-bismuth/historiography/utils"
	"github.com/spf13/cobra"
	git "gopkg.in/libgit2/git2go.v25"
)

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

func IteratorFunc(commit *git.Commit) bool {
	glog.Infof(commit.Summary())
	return true
}

func WalkRepo(repo *git.Repository) {
	if rev, err := repo.Walk(); err != nil {
		utils.MustG(err)
	} else {
		defer rev.Free()
		rev.Sorting(git.SortTime)
		utils.MustG(rev.PushHead())
		utils.MustG(rev.Iterate(IteratorFunc))
	}
}

func init() {
	utils.InitCli(root)
}

func main() {
	utils.Must(root.Execute())
}
