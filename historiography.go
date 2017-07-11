package main

import (
	"github.com/backinmydays/historiography/utils"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	git "gopkg.in/libgit2/git2go.v25"
)

var root = &cobra.Command{
	Use:   "historiography",
	Short: "Rewrite git history dates",
	Run: func(cmd *cobra.Command, args []string) {
		repos := []*git.Repository{}

		for _, arg := range args {
			if repo, err := git.OpenRepository(arg); err != nil {
				utils.MustG(err)
			} else {
				repos = append(repos, repo)
			}
		}
		glog.Infof("%q", repos)
	},
}

func init() {
	utils.InitCli(root)
}

func main() {
	utils.Must(root.Execute())
}
