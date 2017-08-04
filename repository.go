package historiography

import (
	"errors"
	git "gopkg.in/libgit2/git2go.v26"
	"time"
)

// Interface used to describe an iterator used by a git RevWalk function inside
// the historiography package. It basically goes over commits a reversed topology
// order.
type RevWalkerIterator interface {
	RevWalkIterator(*git.Commit) bool
}

// Implementation of the RevWalkerIterator interface to retrieve commits of a
// branch. This iterator is used to group commits per day and is used by the
// Retrieve function.
type RetrieveIterator struct {
	Commits []Commits
	nb      int
	day     int
	year    int
	month   time.Month
}

// Iterator function, go over commits and store them in an internal structure.
func (ri *RetrieveIterator) RevWalkIterator(commit *git.Commit) bool {
	date := commit.Author().When
	if ri.day != date.Day() || ri.month != date.Month() || ri.year != date.Year() {
		ri.Commits = append([]Commits{Commits{}}, ri.Commits...)
		ri.year, ri.month, ri.day = date.Date()
	}
	ri.Commits[0] = append(ri.Commits[0], commit)

	ri.nb -= 1
	if ri.nb == 0 {
		return false
	} else {
		return true
	}
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
	rev.Sorting(git.SortTopological)

	if err = rev.PushHead(); err != nil {
		return
	}
	return rev.Iterate(rwi.RevWalkIterator)
}

// Retrieve all commits of the current repository branch.
// It internally use RepoWalk with an instance of a RetrieveIterator.
func Retrieve(repo *git.Repository, nb int) ([]Commits, error) {
	ri := RetrieveIterator{nb: nb}
	err := RepoWalk(repo, &ri)

	if err == nil && len(ri.Commits) == 0 {
		err = errors.New("there is not commit to process")
	}
	return ri.Commits, err
}
