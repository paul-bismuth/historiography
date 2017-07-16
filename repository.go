package historiography

import (
	git "gopkg.in/libgit2/git2go.v26"
	"time"
)

func Retrieve(repo *git.Repository) (commits []Commits, err error) {
	day, year, month := 0, 0, time.January

	rev, err := repo.Walk()
	if err != nil {
		return
	}

	defer rev.Free()
	rev.Sorting(git.SortReverse)
	if err = rev.PushHead(); err != nil {
		return
	}
	//if glog.V(1) {
	//	glog.Infof("parsing %s repository", repo.Workdir())
	//}

	err = rev.Iterate(func(commit *git.Commit) bool {
		date := commit.Author().When
		if day != date.Day() || month != date.Month() || year != date.Year() {
			commits = append(commits, Commits{})
			year, month, day = date.Date()
		}
		commits[len(commits)-1] = append(commits[len(commits)-1], commit)

		return true
	})
	return
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
