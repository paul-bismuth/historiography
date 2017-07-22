package historiography

import (
	git "gopkg.in/libgit2/git2go.v26"
	"time"
)

// Interface used to describe an iterator used by a git RevWalk function inside
// the historiography package. It basically goes over commits a reversed topology
// order.
type RevWalkerIterator interface {
	RevWalkIterator(*git.Commit) bool
}

// Implementation of the RevWalkerIterator interface to retrieve the root commit
// of a branch. This iterator stop at the first iteration and store the commit
// internally.
type RetrieveRootIterator struct {
	root *git.Commit
}

// Iterator function, stops at the root commit of a branch and store it internally.
func (rri *RetrieveRootIterator) RevWalkIterator(commit *git.Commit) bool {
	rri.root = commit
	return false
}

// Implementation of the RevWalkerIterator interface to retrieve commits of a
// branch. This iterator is used to group commits per day and is used by the
// Retrieve function.
type RetrieveIterator struct {
	commits []Commits
	day     int
	year    int
	month   time.Month
}

// Iterator function, go over commits and store them in an internal structure.
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
// It use the RevWalkerIterator interface function: RevWalkIterator to walk
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

// Retrieve the root commit of the current repository branch.
// It internally use RepoWalk with an instance of a RetrieveRootIterator.
func RetrieveRoot(repo *git.Repository) (*git.Commit, error) {
	var rri RetrieveRootIterator
	return rri.root, RepoWalk(repo, &rri)
}

// Retrieve all commits of the current repository branch.
// It internally use RepoWalk with an instance of a RetrieveIterator.
func Retrieve(repo *git.Repository) (commits []Commits, err error) {
	var ri RetrieveIterator
	return ri.commits, RepoWalk(repo, &ri)
}

// Reorganise a list of commits according to a distributer strategy.
// This function goes over commits and first check if commits of the day need
// to be reschedule, and if so, ask a distribute function to provide needed
// changes. Nothing is done here, this just precompute what the new dates can
// be.
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
