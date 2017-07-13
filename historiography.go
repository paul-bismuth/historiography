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
type Changes map[git.Oid]time.Time

func branch() string {
	return "test"
	//return utils.SecureRandomString(branchNameSize)
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

func push(hour int, commit *git.Commit, changes Changes) {
	old := commit.Author().When

	if glog.V(2) {
		glog.Infof(
			"commit: %s pushing from %d:%d to %d:%d",
			commit.Id().String()[:10], old.Hour(), old.Minute(), hour, old.Minute(),
		)
	}
	changes[*commit.Id()] = time.Date(
		old.Year(), old.Month(), old.Day(), hour, old.Minute(), old.Second(),
		old.Nanosecond(), old.Location(),
	)
}

func distribute(commits []*git.Commit, changes Changes) {
	i := 0

	// reverse ordered commits
	for _, commit := range commits {
		if startHour > commit.Author().When.Hour() {
			break
		}
		i++
	}

	if i == len(commits) { // we can place commits in the morning between 8-9 AM
		i = i - 1
		prev := commits[i].Author().When

		for ; i >= 0; i-- {
			commit := commits[i]
			date := commit.Author().When
			if date.Hour() < 13 && prev.Hour() == date.Hour() {
				push(8, commit, changes)
				prev = date
			} else {
				break
			}
		}
	}
	// calculate remaining elapsed time
	end := commits[0].Author().When
	start := commits[i].Author().When
	elapsed := end.Sub(start).Hours()
	if glog.V(2) {
		glog.Infof("elapsed time between remaining commits: %.2f hours", elapsed)
	}
}

func reorganise(commits [][]*git.Commit) ([]*git.Commit, Changes) {

	//if glog.V(2) {
	//	glog.Infof("%q", commits)
	//}
	changes := make(Changes)
	reordered := make([]*git.Commit, 0)

	for i := len(commits) - 1; i >= 0; i-- {
		day := commits[i][0].Author().When
		if glog.V(1) {
			glog.Infof("computed day: %s", day.Format("Mon 02 Jan 2006"))
		}

		if d := day.Weekday(); d != 0 && d != 6 {
			distribute(commits[i], changes)
		}

		for j := len(commits[i]) - 1; j >= 0; j-- {
			reordered = append(reordered, commits[i][j])
		}
	}
	return reordered, changes
}

func rebase(
	repo *git.Repository, commits []*git.Commit, changes Changes,
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

	r, m, a, c, t, err := getArgs(options, parent, changes)
	if err != nil {
		return
	}
	id, err := parent.Amend(r, a, c, m, t)
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

func getArgs(
	options *Options, commit *git.Commit, changes Changes,
) (
	r, m string, a, c *git.Signature, t *git.Tree, e error,
) {
	r, m = options.Ref(), commit.RawMessage()
	a, c = commit.Author(), commit.Committer()
	if date, ok := changes[*commit.Id()]; ok {
		a.When, c.When = date, date
	}
	t, e = commit.Tree()
	return
}

func amend(
	options *Options, commit, parent *git.Commit, changes Changes,
) (
	*git.Commit, error,
) {
	err := options.repo.Cherrypick(commit, options.cherrypick)
	if err != nil {
		return nil, err
	}

	r, m, a, c, t, err := getArgs(options, commit, changes)
	if err != nil {
		return nil, err
	}

	id, err := options.repo.CreateCommit(r, a, c, m, t, parent)
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
}

func init() {
	utils.InitCli(root)
}

func main() {
	utils.Maybe(root.Execute())
}
