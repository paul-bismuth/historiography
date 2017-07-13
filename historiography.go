package main

import (
	"github.com/golang/glog"
	"github.com/paul-bismuth/historiography/utils"
	"github.com/spf13/cobra"
	git "gopkg.in/libgit2/git2go.v26"
	"time"
)

const format = "20060102"
const branchNameSize = 8

// opening hours 9h - 18h -> repartition goes from 8h - 9h, 18h - 00h
const startHour = 9
const endHour = 18

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

func newHour(hour int, old time.Time) time.Time {
	return time.Date(
		old.Year(), old.Month(), old.Day(), hour, old.Minute(), old.Second(),
		old.Nanosecond(), old.Location(),
	)
}

func distribute(commits []*git.Commit) map[git.Oid]time.Time {
	i, j := 0, 0
	changes := make(map[git.Oid]time.Time)

	// reverse ordered commits
	for _, commit := range commits {
		date := commit.Author().When.Hour()
		if startHour > date {
			break
		}
		if endHour <= date {
			i++
			j++
			continue
		}
		j++
	}
	if j == len(commits) { // we can place commit in the morning between 8-9 AM
		for j = j - 1; j >= i; j-- {
			commit := commits[j]
			date := commit.Author().When
			if date.Hour() < 12 && date.Minute() < 30 { // CHANGE THIS!
				changes[*commit.Id()] = newHour(8, date)
			}
		}
	}
	return changes

	//if glog.V(1) {
	//	glog.Infof("elapsed time between commits: %.2f hours", end.Sub(start).Hours())
	//}
}

func reorganise(commits [][]*git.Commit) ([]*git.Commit, map[git.Oid]time.Time) {

	if glog.V(2) {
		glog.Infof("%q", commits)
	}
	changes := make(map[git.Oid]time.Time)
	reordered := make([]*git.Commit, 0)

	for i := len(commits) - 1; i >= 0; i-- {
		day := commits[i][0].Author().When
		if glog.V(1) {
			glog.Infof("computed day: %s", day.Format("Mon 02 Jan 2006"))
		}

		if d := day.Weekday(); d != 0 && d != 6 {
			for k, v := range distribute(commits[i]) {
				changes[k] = v
			}
		}

		for j := len(commits[i]) - 1; j >= 0; j-- {
			reordered = append(reordered, commits[i][j])
		}
	}
	return reordered, changes
}

func changeDates(
	commit *git.Commit, changes map[git.Oid]time.Time,
) (
	author, committer *git.Signature,
) {
	author, committer = commit.Author(), commit.Committer()
	if newdate, ok := changes[*commit.Id()]; ok {
		author.When, committer.When = newdate, newdate
	}
	return
}

func rebase(
	repo *git.Repository, commits []*git.Commit, changes map[git.Oid]time.Time,
) (err error) {
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
	t, err := parent.Tree()
	if err != nil {
		return
	}
	a, c := changeDates(parent, changes)
	id, err := parent.Amend(options.Ref(), a, c, parent.RawMessage(), t)
	parent, err = repo.LookupCommit(id)
	if err != nil {
		return
	}
	for _, commit := range commits[1:] {
		parent, err = amend(options, commit, parent, changes)
		if err != nil {
			return
		}
	}

	return repo.StateCleanup()
}

func amend(
	options *Options, commit, parent *git.Commit, changes map[git.Oid]time.Time,
) (
	*git.Commit, error,
) {
	err := options.repo.Cherrypick(commit, options.cherrypick)
	if err != nil {
		return nil, err
	}
	t, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	id, err := options.repo.CreateCommit(
		options.Ref(), commit.Author(), commit.Committer(), commit.RawMessage(), t, parent,
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

	reordered, changes := reorganise(commits)
	return rebase(repo, reordered, changes)
	//return nil
}

func init() {
	utils.InitCli(root)
}

func main() {
	utils.Maybe(root.Execute())
}
