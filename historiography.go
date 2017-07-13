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
	new := old.Add(time.Duration(hour-old.Hour()) * time.Hour)

	if glog.V(2) {
		glog.Infof(
			"commit: %s pushing from %s to %s",
			commit.Id().String()[:10], old.Format("15:04"), new.Format("15:04"),
		)
	}
	changes[*commit.Id()] = new
}

func distribute(commits []*git.Commit, changes Changes) {
	tmp := [28][]*git.Commit{}
	empty := []*git.Commit{}

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
	for i := 9; i < 24; i++ {
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
	if first > 9 || last < 18 {
		// remaining elapsed time
		elapsed := last - first
		step := utils.Intn(26-18-elapsed) + 18

		if glog.V(2) {
			glog.Infof("elapsed time of commits chunk %d hours", elapsed)
		}
		for ; last >= first; last-- {
			tmp[step], tmp[last] = tmp[last], empty
			step++
		}
	}

	for i, commits := range tmp {
		for _, commit := range commits {
			push(i, commit, changes)
		}
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
