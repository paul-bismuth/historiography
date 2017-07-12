package main

import (
	"github.com/backinmydays/historiography/utils"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	git "gopkg.in/libgit2/git2go.v26"
	"time"
)

const format = "20060102"

// protect against dirty repos

type Repository struct {
	*git.Repository
	options git.CherrypickOptions
}

func NewRepository(path string) (repo *Repository, err error) {
	repo = &Repository{}

	if repo.options, err = git.DefaultCherrypickOptions(); err != nil {
		return
	}
	if repo.Repository, err = git.OpenRepository(path); err != nil {
		return
	}

	return
}

var root = &cobra.Command{
	Use:   "historiography",
	Short: "Rewrite git history dates",
	Run: func(cmd *cobra.Command, args []string) {
		for _, arg := range args {
			if repo, err := NewRepository(arg); err != nil {
				utils.Maybe(err)
			} else {
				defer repo.Free()
				utils.Maybe(repo.Reorganise( /* can pass options here in future*/ ))
			}
		}
	},
}

func reorganise(commits [][]*git.Commit) (res []*git.Commit) {

	if glog.V(2) {
		glog.Infof("%q", commits)
	}

	// commits are in reverse order
	for i := len(commits) - 1; i >= 0; i-- {
		start := commits[i][len(commits[i])-1].Author().When
		end := commits[i][0].Author().When

		if glog.V(1) {
			glog.Infof("computed day: %s", start.Format("Mon 02 Jan 2006"))
			glog.Infof("elapsed time between commits: %f seconds", end.Sub(start).Seconds())
		}
		if day := start.Weekday(); day != 0 || day != 6 {
			//recompute here
		}

		for j := len(commits[i]) - 1; j >= 0; j-- {
			res = append(res, commits[i][j])
		}
	}
	return
}

func (repo *Repository) rebase(commits []*git.Commit) (err error) {
	// libgit does not support interactive rebase
	// https://github.com/libgit2/libgit2/pull/2482
	// we will create a temp branch and cherry pick from
	if len(commits) == 0 {
		return nil
	}

	parent := commits[0]

	b, err := repo.CreateBranch("test", parent, false)
	if err != nil {
		return
	}

	err = repo.SetHead(b.Reference.Name())
	if err != nil {
		return
	}
	err = repo.CheckoutHead(&git.CheckoutOpts{Strategy: git.CheckoutForce})
	if err != nil {
		return
	}

	for _, commit := range commits[1:] {
		parent, err = repo.amend(b, commit, parent)
		if err != nil {
			return
		}
	}

	return
}

func (repo *Repository) amend(branch *git.Branch, commit, parent *git.Commit) (*git.Commit, error) {
	err := repo.Cherrypick(commit, repo.options)
	if err != nil {
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	id, err := repo.CreateCommit(
		branch.Reference.Name(), commit.Author(), commit.Committer(), "prout",
		tree, parent,
	)
	obj, err := repo.LookupCommit(id)
	if err != nil {
		return nil, err
	}
	return obj.AsCommit()
}

func (repo *Repository) Reorganise() error {
	day, year, month := 0, 0, time.January
	commits := [][]*git.Commit{}

	rev, err := repo.Walk()
	if err != nil {
		return err
	}
	defer rev.Free()
	rev.Sorting(git.SortTime)
	if err := rev.PushHead(); err != nil {
		return err
	}
	glog.Infof("parsing %s repository", repo.Workdir())

	if err := rev.Iterate(func(commit *git.Commit) bool {
		date := commit.Author().When
		if day != date.Day() || month != date.Month() || year != date.Year() {
			commits = append(commits, []*git.Commit{})
			year, month, day = date.Date()
		}
		commits[len(commits)-1] = append(commits[len(commits)-1], commit)

		return true
	}); err != nil {
		return err
	}
	return repo.rebase(reorganise(commits))
}

func init() {
	utils.InitCli(root)
}

func main() {
	utils.Must(root.Execute())
}
