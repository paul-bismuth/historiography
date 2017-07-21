package historiography

import (
	git "gopkg.in/libgit2/git2go.v26"
	"time"
)

type RevWalkerIterator interface {
	RevWalkIterator(*git.Commit) bool
}

type RetrieveRootIterator struct {
	root *git.Commit
}

func (rri *RetrieveRootIterator) RevWalkIterator(commit *git.Commit) bool {
	rri.root = commit
	return false
}

type RetrieveIterator struct {
	commits []Commits
	day     int
	year    int
	month   time.Month
}

func (ri *RetrieveIterator) RevWalkIterator(commit *git.Commit) bool {
	date := commit.Author().When
	if ri.day != date.Day() || ri.month != date.Month() || ri.year != date.Year() {
		ri.commits = append(ri.commits, Commits{})
		ri.year, ri.month, ri.day = date.Date()
	}
	index := len(ri.commits) - 1
	ri.commits[index] = append(ri.commits[index], commit)

	return true
}

// Walk throught commits of a repo using RevWalk from libgit.
// Commits are passed in reversed topologically order (parent first,
// then children). The RevWalk is started over HEAD refs.
// It use the RevWalkerIterator interface function RevWalkIterator to walk
// over commits.
func RepoWalk(repo *git.Repository, rwi RevWalkerIterator) (err error) {
	var rev *git.RevWalk

	if rev, err = repo.Walk(); err != nil {
		return
	}
	defer rev.Free()
	rev.Sorting(git.SortTopological | git.SortReverse)

	if err = rev.PushHead(); err != nil {
		return
	}
	return rev.Iterate(rwi.RevWalkIterator)
}

func RetrieveRoot(repo *git.Repository) (*git.Commit, error) {
	var rri RetrieveRootIterator
	return rri.root, RepoWalk(repo, &rri)
}

func Retrieve(repo *git.Repository) (commits []Commits, err error) {
	var ri RetrieveIterator
	return ri.commits, RepoWalk(repo, &ri)
}

func Reorganise(commits []Commits, distributer Distributer) Changes {
	changes := Changes{}

	for _, commit := range commits {
		if distributer.Reschedule(commit) {
			for k, v := range distributer.Distribute(commit) {
				changes[k] = v
			}
		}
	}
	return changes
}
