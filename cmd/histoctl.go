package main

import (
	"fmt"
	"github.com/backinmydays/historiography"
	"github.com/backinmydays/historiography/utils"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	git "gopkg.in/libgit2/git2go.v26"
	"strings"
	"time"
)

const branchNameSize = 8

// opening hours 9h - 18h -> repartition goes from 8h - 9h, 18h - 02h
const startHour = 9
const endHour = 18

// protect against dirty repos
type Changes map[git.Oid]time.Time
type Commits []*git.Commit

func (c Commits) String() string {
	if len(c) == 0 {
		return "{}"
	}
	var date time.Time
	commits := []string{}

	for _, entry := range c {
		date = entry.Author().When
		commits = append(commits, fmt.Sprintf("%s: %s",
			entry.Id().String()[:10], date.Format("15:04")))
	}
	return fmt.Sprintf("{%s: [%s]}", date.Format("2006/01/02"), strings.Join(commits, ", "))
}

func branch() string {
	return utils.SecureRandomString(branchNameSize)
}

// convenient struct to store bunch of params
type Options struct {
	repo       *git.Repository
	head       *git.Reference
	branch     *git.Branch
	checkout   git.CheckoutOpts
	cherrypick git.CherrypickOptions
}

func (o *Options) Override() error {
	ref, err := o.branch.Resolve()
	if err != nil {
		return err
	}
	commit, err := o.repo.LookupCommit(ref.Target())
	if err != nil {
		return err
	}
	branch, err := o.head.Branch().Name()
	if err != nil {
		return err
	}
	_, err = o.repo.CreateBranch(branch, commit, true)
	return err
}

func (o *Options) Ref() string {
	return o.branch.Reference.Name()
}

func (o *Options) Delete() (err error) {
	var ref *git.Reference

	if ref, err = o.branch.Resolve(); err != nil {
		return nil // branch does not exist anymore, abort
	}
	if err = o.repo.SetHead(o.head.Name()); err != nil {
		return
	}
	if err = o.repo.CheckoutHead(&o.checkout); err != nil {
		return
	}
	if err = ref.Delete(); err != nil {
		return
	}
	return
}

func (o *Options) Free() {
	if err := o.repo.StateCleanup(); err != nil {
		glog.Errorf("cleaning repo state failed: %s", err)
	}
	if err := o.Delete(); err != nil {
		glog.Errorf("cleaning repo state failed: %s", err)
	}

	o.branch.Reference.Free()
}

func NewOptions(repo *git.Repository, target *git.Commit) (opt *Options, err error) {
	opt = &Options{repo: repo}

	if repo.State() != git.RepositoryStateNone {
		return nil, fmt.Errorf("repository is not in a clear state")
	}

	opt.cherrypick, err = git.DefaultCherrypickOptions()
	if err != nil {
		return
	}

	opt.head, err = repo.Head()
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
	Use:   "histoctl",
	Short: "Rewrite git history dates",
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cmd.SilenceUsage = true
		var repo *git.Repository

		for _, arg := range args {
			if repo, err = git.OpenRepository(arg); err != nil {
				return
			}

			defer repo.Free()

			if err = Reorganise(repo); err != nil {
				return
			}
		}
		return
	},
}

func push(hour int, commit *git.Commit, changes Changes) {
	old := commit.Author().When
	new := old.Add(time.Duration(hour-old.Hour()) * time.Hour)

	if glog.V(2) {
		glog.Infof(
			"commit: %s pushing from %s to %s",
			commit.Id().String()[:10], old.Format("15:04"), new.Format("15:04"),
		)
	}
	changes[*commit.Id()] = new
}

func distribute(commits Commits, changes Changes) {
	tmp := [28]Commits{}
	empty := Commits{}
	// repartition function
	repartition := utils.Weighted(10, 8, 4, 2)

	for i := len(commits) - 1; i >= 0; i-- { // commits in reverse order
		hour := commits[i].Author().When.Hour()
		tmp[hour] = append(tmp[hour], commits[i])
	}

	// check if 8-9 is empty and push commits there if so
	if len(tmp[8]) == 0 {
		// randomly pick the end of the scan
		for i := 8; i < 8+utils.Intn(8); i++ {
			if len(tmp[i]) != 0 {
				tmp[8], tmp[i] = tmp[i], empty
			}
		}
	}

	// reduce time between commits to max 2 hours
	first, last := 0, 0
	for i := startHour; i < 24; i++ {
		if len(tmp[i]) != 0 { // if there's commits
			if first == 0 { // init if not already done
				first, last = i, i
			} else if i-last < 3 { // if difference if less than 2 hours
				last = i // do nothing and update last encountered
			} else { // otherwise move commits to reduce diff
				tmp[last+2], tmp[i] = tmp[i], empty
				last = last + 2
			}
		}
	}
	if first >= startHour || last < endHour {
		// remaining elapsed time
		elapsed := last - first
		step := 18 - first + repartition()

		if glog.V(2) {
			glog.Infof("elapsed time of commits chunk %d hours", elapsed)
		}
		for ; last >= first; last-- {
			tmp[last+step], tmp[last] = tmp[last], empty
		}
	}

	for i, commits := range tmp {
		for _, commit := range commits {
			push(i, commit, changes)
		}
	}
}

func reorganise(commits []Commits) (Commits, Changes) {
	if glog.V(5) {
		glog.Infof("%q", commits)
	}
	changes := make(Changes)
	reordered := make(Commits, 0)

	for i := len(commits) - 1; i >= 0; i-- {
		day := commits[i][0].Author().When
		if glog.V(1) {
			glog.Infof("computing day: %s", day.Format("Mon 02 Jan 2006"))
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
	repo *git.Repository, commits Commits, changes Changes,
) (
	err error,
) {
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
	if err != nil {
		return
	}

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

	ok, err := historiography.Confirm(options.repo)
	if err != nil || !ok {
		return
	}
	return options.Override()
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
	commits := []Commits{}

	rev, err := repo.Walk()
	if err != nil {
		return err
	}
	defer rev.Free()
	rev.Sorting(git.SortTime)
	if err := rev.PushHead(); err != nil {
		return err
	}
	if glog.V(1) {
		glog.Infof("parsing %s repository", repo.Workdir())
	}

	if err := rev.Iterate(func(commit *git.Commit) bool {
		date := commit.Author().When
		if day != date.Day() || month != date.Month() || year != date.Year() {
			commits = append(commits, Commits{})
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
	pred := func(err error) bool {
		return (err != nil && bool(glog.V(1)) &&
			!strings.Contains(err.Error(), "unknown flag:"))
	}
	if err := root.Execute(); pred(err) {
		glog.Errorf("%s", err)
	}
}
