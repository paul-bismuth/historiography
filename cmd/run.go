package main

import (
	"github.com/golang/glog"
	histo "github.com/paul-bismuth/historiography"
	git "gopkg.in/libgit2/git2go.v26"
	"time"
)

// opening hours 9h - 18h -> repartition goes from 8h - 9h, 18h - 02h
const startHour = 9
const endHour = 18

var closedDays = []time.Weekday{time.Saturday, time.Sunday}

func newComposerProcessor(name, email string) *histo.ComposerProcessor {
	// init processors
	processors := []histo.Processer{
		&histo.DateProcessor{closedDays, startHour, endHour, make(map[git.Oid]time.Time)},
	}

	// add more processors if needed
	if name != "" {
		processors = append(processors, &histo.NameProcessor{name})
	}
	if email != "" {
		processors = append(processors, &histo.EmailProcessor{email})
	}
	return &histo.ComposerProcessor{processors}
}

func filter(commits histo.Commits, start int) []histo.Commits {
	var iterator histo.RetrieveIterator
	for i := start; i < len(commits); i++ {
		iterator.RevWalkIterator(commits[i])
	}
	return iterator.Commits
}

func run(args []string, nb int, name, email string) (err error) {
	var repo *git.Repository
	var commits []histo.Commits
	var historiography *histo.Historiography

	processor := newComposerProcessor(name, email)
	for _, arg := range args {
		if repo, err = git.OpenRepository(arg); err != nil {
			return
		}

		if glog.V(1) {
			glog.Infof("parsing %s repository", repo.Workdir())
		}

		defer repo.Free()
		if glog.V(5) {
			glog.Infof("%q", commits) // display all commits retrieved in debug mode
		}

		// init historiography struct
		if historiography, err = histo.NewHistoriography(repo, processor, nb); err != nil {
			return
		}
		// be sure to free resources when ending
		defer historiography.Free()

		commits = historiography.Commits

		// infers changes needed to be in sync with the distribution strategy
		if err = historiography.Preprocess(commits); err != nil {
			return
		}

		// logs changes in a convenient if verbosity is high enough
		if glog.V(2) {
			if dp, ok := processor.Processors[0].(*histo.DateProcessor); ok {
				logs(commits, dp)
			}
		}

		// apply changes on the temporary branch
		if err = historiography.Process(histo.Flatten(commits)); err != nil {
			return
		}

		// ask for confirmation if needed and override branch
		if err = confirm(historiography, repo); err != nil {
			return
		}
	}
	return
}

// Wrapper for the confirmation, call override from historiography object if
// user validate changes, or directly override if force flag has been passed.
func confirm(h *histo.Historiography, repo *git.Repository) (err error) {
	ok := force // not a good design has to be improved
	if !ok {
		ok, err = histo.Confirm(repo)
	}
	if ok {
		err = h.Override()
	}
	return
}

// Logs commits and changes through glog in a readable way.
func logs(commits []histo.Commits, p *histo.DateProcessor) {
	fmt := func(t time.Time) string { return t.Format("15:04") } // all times formatted the same way

	for _, day := range commits {
		// there can't be empty day, at least one commit will exists
		d := day[0].Author().When
		glog.Infof("computing day: %s", d.Format("Mon 02 Jan 2006"))

		for _, commit := range day {
			commitTime := commit.Author().When
			id := commit.Id().String()[:10]
			if changeTime, ok := p.Changes[*commit.Id()]; ok {
				glog.Infof("commit %s at %s changed to %s", id, fmt(commitTime), fmt(changeTime))
			} else {
				glog.Infof("commit %s at %s not changed", id, fmt(commitTime))
			}
		}
	}
}
