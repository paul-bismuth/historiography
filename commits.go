package historiography

import (
	"fmt"
	git "gopkg.in/libgit2/git2go.v26"
	"strings"
	"time"
)

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

// Define a strategy to alter commits.
type Processer interface {
	// First phase to run, allow user to have an overview of all commits of a
	// branch and perform some preprocessing.
	Preprocess(Commits) error
	// Called when applying a commit, allow user to override some informations.
	Process(*git.Commit) (a, c *git.Signature, m string, e error)
}

// Default processer for commits implemented in this library.
type Processor struct {
	// Closed day in which commit hours are not relevant, those days commits will
	// not be rescheduled.
	Closed []time.Weekday
	// If we are not in a closed day, we want to move commits out of a certain time
	// frame. Start and End, represent the limits of this time frame. For example:
	// we do not want commits to occur between 9h and 18h, we set up Start to
	// 9 and End to 18.
	Start, End int
	// Stores the id of commit and the new time to apply when rewriting.
	Changes map[git.Oid]time.Time
}

// Implementation for default distributer.
// Indicates if the day need a rescheduling,
// i.e: commits are between Start and End hours and out of Closed days.
func (p *Processor) Preprocess(commits Commits) (_ error) {
	// if empty no need to reschedule the day
	if len(commits) == 0 {
		return
	}
	// first check day of commit list, if in closed days no need to reschedule
	day := commits[0].Author().When.Weekday()
	for _, o := range p.Closed {
		if o == day {
			return
		}
	}

	// now check if some commits are between start and end
	for _, commit := range commits {
		hour := commit.Author().When.Hour()
		if hour >= p.Start && hour < p.End {
			p.Distribute(commits)
		}
	}
	return
}

// Distribute day commits out of the Start and End limit.
// Introduce some random distribution mechanisms to avoid pushing to the same
// hours accross days.
func (p *Processor) Distribute(commits Commits) {
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
	for i := p.Start; i < 24; i++ {
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
	if first >= p.Start || last < p.End {
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

			p.Changes[*commit.Id()] = new
		}
	}
}

func (p *Processor) Process(commit *git.Commit) (a, c *git.Signature, m string, e error) {
	m = commit.RawMessage()
	a, c = commit.Author(), commit.Committer()
	// if we spot a change on this commit, we update the dates to match change.
	if date, ok := p.Changes[*commit.Id()]; ok {
		a.When, c.When = date, date
	}
	return
}
