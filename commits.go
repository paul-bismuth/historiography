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

// Processor for changing dates of a commit list.
type DateProcessor struct {
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
func (dp *DateProcessor) Preprocess(commits Commits) (_ error) {
	// if empty no need to reschedule the day
	if len(commits) == 0 {
		return
	}
	// first check day of commit list, if in closed days no need to reschedule
	day := commits[0].Author().When.Weekday()
	for _, o := range dp.Closed {
		if o == day {
			return
		}
	}

	// now check if some commits are between start and end
	for _, commit := range commits {
		hour := commit.Author().When.Hour()
		if hour >= dp.Start && hour < dp.End {
			dp.Distribute(commits)
		}
	}
	return
}

// Distribute day commits out of the Start and End limit.
// Introduce some random distribution mechanisms to avoid pushing to the same
// hours accross days.
func (dp *DateProcessor) Distribute(commits Commits) {
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
	for i := dp.Start; i < 24; i++ {
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
	if first >= dp.Start || last < dp.End {
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

			dp.Changes[*commit.Id()] = new
		}
	}
}

// Process a commit and return changed values if needed. DateProcessor only
// operate on commit dates. Author and committer will not be changed.
func (dp *DateProcessor) Process(commit *git.Commit) (a, c *git.Signature, m string, e error) {
	m = commit.RawMessage()
	a, c = commit.Author(), commit.Committer()
	// if we spot a change on this commit, we update the dates to match change.
	if date, ok := dp.Changes[*commit.Id()]; ok {
		a.When, c.When = date, date
	}

	return
}

// This processor replace name for both author and committer in all commits
// processed.
type NameProcessor struct {
	// New name to apply on commits.
	Name string
}

// Preprocess is no-op for NameProcessor
func (np *NameProcessor) Preprocess(_ Commits) error { return nil }

// Change author and committer name to the new one defined by the structure.
func (np *NameProcessor) Process(commit *git.Commit) (a, c *git.Signature, m string, e error) {
	m = commit.RawMessage()
	a, c = commit.Author(), commit.Committer()
	a.Name, c.Name = np.Name, np.Name

	return
}

// This processor replace email for both author and committer in all commits
// processed.
type EmailProcessor struct {
	// New email to apply on commits.
	Email string
}

// Preprocess is no-op for EmailProcessor
func (ep *EmailProcessor) Preprocess(_ Commits) error { return nil }

// Change author and committer email to the new one defined by the structure.
func (ep *EmailProcessor) Process(commit *git.Commit) (a, c *git.Signature, m string, e error) {
	m = commit.RawMessage()
	a, c = commit.Author(), commit.Committer()
	a.Email, c.Email = ep.Email, ep.Email

	return
}

// Processor for composing multiple processors, order matter especially if
// there is possibilities for override. ComposerProcessor is also a Processer
// and run Preprocess/Process of embedded Processer in order of appearance.
type ComposerProcessor struct {
	// List of processors to apply on each commit.
	Processors []Processer
}

// Run Preprocess of embedded Processer in order of appearance.
func (cp *ComposerProcessor) Preprocess(commits Commits) error {
	for _, processor := range cp.Processors {
		if err := processor.Preprocess(commits); err != nil {
			return err
		}
	}
	return nil
}

// Push changes from new to res only if new is different from old entry.
func mergeSignature(res, new, old *git.Signature) {
	if old.When != new.When {
		res.When = new.When
	}
	if old.Name != new.Name {
		res.Name = new.Name
	}
	if old.Email != new.Email {
		res.Email = new.Email
	}
}

// Run Process of embedded processers in order of appearance. Note that if
// there is a difference with the base commit, only the last one is kept.
// Override is performed in order of appearance, i.e:
//	Processors: [processer_1, processer_2]
//
//	Process -> processer_1.Process -> processer_2.Process
//
func (pc *ComposerProcessor) Process(commit *git.Commit) (a, c *git.Signature, m string, e error) {
	m = commit.RawMessage()
	a, c = commit.Author(), commit.Committer()
	for _, processor := range pc.Processors {
		_a, _c, _m, _e := processor.Process(commit)
		if e = _e; e != nil {
			return
		}
		if m != _m {
			m = _m
		}
		mergeSignature(a, _a, commit.Author())
		mergeSignature(c, _c, commit.Committer())
	}
	return
}
