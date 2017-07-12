package main

import (
	"github.com/backinmydays/historiography/utils"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	git "gopkg.in/libgit2/git2go.v26"
	"time"
)

const format = "20060102"
const branchNameSize = 8

// protect against dirty repos

func branch() string {
	return utils.SecureRandomString(branchNameSize)
}

// convenient struct to store bunch of params
type Options struct {
	repo       *git.Repository
	branch     *git.Branch
	checkout   git.CheckoutOpts
	cherrypick git.CherrypickOptions
}

func (o *Options) Ref() string {
	return o.branch.Reference.Name()
}

func (o *Options) Free() {
	o.branch.Reference.Free()
}

func NewOptions(repo *git.Repository, target *git.Commit) (opt *Options, err error) {
	opt = &Options{repo: repo}
	opt.cherrypick, err = git.DefaultCherrypickOptions()
	if err != nil {
		return
	}
	for {
		opt.branch, err = repo.CreateBranch(branch(), target, false)
		if err == nil {
			break
		}
	}

	opt.checkout = git.CheckoutOpts{Strategy: git.CheckoutForce}
	return
}

var root = &cobra.Command{
	Use:   "historiography",
	Short: "Rewrite git history dates",
	Run: func(cmd *cobra.Command, args []string) {
		for _, arg := range args {
			if repo, err := git.OpenRepository(arg); err != nil {
				utils.Maybe(err)
			} else {
				defer repo.Free()
				utils.Maybe(Reorganise(repo))
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

func rebase(repo *git.Repository, commits []*git.Commit) (err error) {
	// libgit does not support interactive rebase
	// https://github.com/libgit2/libgit2/pull/2482
	// we will create a temp branch and cherry pick from
	if len(commits) == 0 {
		return nil
	}

	parent := commits[0]
	options, err := NewOptions(repo, parent)
	if err != nil {
		return
	}
	defer options.Free()

	err = repo.SetHead(options.Ref())
	if err != nil {
		return
	}
	err = repo.CheckoutHead(&options.checkout)
	if err != nil {
		return
	}
	tree, err := parent.Tree()
	if err != nil {
		return
	}
	id, err := parent.Amend(
		options.Ref(), parent.Author(), parent.Committer(), parent.RawMessage(), tree,
	)
	parent, err = repo.LookupCommit(id)
	if err != nil {
		return
	}
	for _, commit := range commits[1:] {
		parent, err = amend(options, commit, parent)
		if err != nil {
			return
		}
	}

	return repo.StateCleanup()
}

func amend(options *Options, commit, parent *git.Commit) (*git.Commit, error) {
	err := options.repo.Cherrypick(commit, options.cherrypick)
	if err != nil {
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	id, err := options.repo.CreateCommit(
		options.Ref(), commit.Author(), commit.Committer(), commit.RawMessage(), tree, parent,
	)
	return options.repo.LookupCommit(id)
}

func Reorganise(repo *git.Repository) error {
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
	return rebase(repo, reorganise(commits))
}

func init() {
	utils.InitCli(root)
}

func main() {
	utils.Maybe(root.Execute())
}
