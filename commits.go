package historiography

import (
	"fmt"
	git "gopkg.in/libgit2/git2go.v26"
	"strings"
	"time"
)

// Stores the id of commit and the new time to apply when rewriting.
type Changes map[git.Oid]time.Time

// Convenient alias for list of commit,
// primarily designed to display debug informations in a convenient way.
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

// Flatten the list of days which contains a list of commits each in a simple
// commit list.
func Flatten(commits []Commits) (flat Commits) {
	for _, i := range commits {
		for _, j := range i {
			flat = append(flat, j)
		}
	}
	return
}

// Define a strategy to reschedule and distribute commits.
type Distributer interface {
	// Add a day worth of commit to be rescheduled, some days may not need any
	// rescheduling because commits are already well distributed or day does not
	// belong to the scope i.e: week-end days for instance.
	Reschedule(Commits) bool
	// Distribute commit accross the day, all commits which should be moved must
	// appear in the Changes structure, with the new date to reschedule to.
	Distribute(Commits) Changes
}

// Default distributer for commits implemented in this library.
type Distribute struct {
	// Closed day in which commit hours are not relevant, those days commits will
	// not be rescheduled.
	Closed []time.Weekday
	// If we are not in a closed day, we want to move commits out of a certain time
	// frame. Start and End, represent the limits of this time frame. For example:
	// we do not want commits to occur between 9h and 18h, we set up Start to
	// 9 and End to 18.
	Start, End int
}

// Implementation for default distributer.
// Indicates if the day need a rescheduling,
// i.e: commits are between Start and End hours and out of Closed days.
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

// Distribute day commits out of the Start and End limit.
// Introduce some random distribution mechanisms to avoid pushing to the same
// hours accross days.
func (d *Distribute) Distribute(commits Commits) Changes {
	changes := make(Changes)
	tmp := [28]Commits{}
	empty := Commits{}

	// repartition function
	repartition := Weighted(10, 8, 4, 2)

	for _, commit := range commits { // commits in reverse order
		hour := commit.Author().When.Hour()
		tmp[hour] = append(tmp[hour], commit)
	}

	// check if 8-9 is empty and push commits there if so
	if len(tmp[8]) == 0 {
		// randomly pick the end of the scan
		for i := 8; i < 8+Intn(8); i++ {
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
	// if there is still need for a rescheduling push commits out of time constraints
	if first >= d.Start || last < d.End {
		for ; last >= first; last-- {
			tmp[last+18-first+repartition()], tmp[last] = tmp[last], empty
		}
	}

	// everything as been rescheduled inside our tmp struct,
	// push all changes to the changes map for use during rewriting phase.
	for i, commits := range tmp {
		for _, commit := range commits {
			old := commit.Author().When
			new := old.Add(time.Duration(i-old.Hour()) * time.Hour)

			changes[*commit.Id()] = new
		}
	}
	return changes
}
