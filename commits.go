package historiography

import (
	"fmt"
	"github.com/backinmydays/historiography/utils"
	"github.com/golang/glog"
	git "gopkg.in/libgit2/git2go.v26"
	"strings"
	"time"
)

// Type storing the id of commit and the new time to apply when rebasing
type Changes map[git.Oid]time.Time

// Convenient alias for list of commit,
// primarely designed to better displayed debug informations.
type Commits []*git.Commit

// Display list of commits.
// Shows first the date of the commit list, then the list of commit ids with hour
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

func Flatten(commits []Commits) (flat Commits) {
	for _, i := range commits {
		for _, j := range i {
			flat = append(flat, j)
		}
	}
	return
}

type Distributer interface {
	// Add a day worth of commit to be rescheduled, some days may not need any
	// rescheduling because commits are already well distributed or day does not
	// to the scope i.e: week-end days for instance
	Reschedule(Commits) bool
	// Distribute commit accross the day, all commits which should be moved must
	// appear in the Changes structure, with the new date to reschedule to.
	Distribute(Commits) Changes
}

type Distribute struct {
	Start, End int
	Closed     []time.Weekday
}

func (d *Distribute) Reschedule(commits Commits) (b bool) {
	// if empty no need to reschedule the day
	if len(commits) == 0 {
		return
	}
	// first check day of commit list, if in closed days no need to reschedule
	day := commits[0].Author().When.Weekday()
	for _, o := range d.Closed {
		if o == day {
			return
		}
	}

	// now check if some commits are between start and end
	for _, commit := range commits {
		hour := commit.Author().When.Hour()
		if hour >= d.Start && hour < d.End {
			return true
		}
	}
	return
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

func (d *Distribute) Distribute(commits Commits) Changes {
	changes := make(Changes)
	tmp := [28]Commits{}
	empty := Commits{}

	// repartition function
	repartition := utils.Weighted(10, 8, 4, 2)

	for _, commit := range commits { // commits in reverse order
		hour := commit.Author().When.Hour()
		tmp[hour] = append(tmp[hour], commit)
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
	for i := d.Start; i < 24; i++ {
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
	if first >= d.Start || last < d.End {
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
	return changes
}

//func reorganise(commits []Commits) (Commits, Changes) {
//	if glog.V(5) {
//		glog.Infof("%q", commits)
//	}
//	changes := make(Changes)
//	reordered := make(Commits, 0)
//
//	for i := len(commits) - 1; i >= 0; i-- {
//		day := commits[i][0].Author().When
//		if glog.V(1) {
//			glog.Infof("computing day: %s", day.Format("Mon 02 Jan 2006"))
//		}
//
//		if d := day.Weekday(); d != 0 && d != 6 {
//			distribute(commits[i], changes)
//		}
//
//		for j := len(commits[i]) - 1; j >= 0; j-- {
//			reordered = append(reordered, commits[i][j])
//		}
//	}
//	return reordered, changes
//}
