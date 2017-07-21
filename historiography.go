package historiography

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/paul-bismuth/historiography/utils"
	git "gopkg.in/libgit2/git2go.v26"
)

const branchNameSize = 8

func tmpBranch(repo *git.Repository) (*git.Reference, error) {
	name := func() string { return SecureRandomString(branchNameSize) }
	root, err := RetrieveRoot(repo)
	if err != nil {
		return nil, err
	}

	for {
		branch, err := repo.CreateBranch(name(), root, false)
		if err == nil {
			return branch.Reference, nil
		}
	}
	return nil, nil // this will never been reached
}

func getArgs(
	h *Historiography, commit *git.Commit, changes Changes,
) (
	r, m string, a, c *git.Signature, t *git.Tree, e error,
) {
	r, m = h.tmp.Name(), commit.RawMessage()
	a, c = commit.Author(), commit.Committer()
	if date, ok := changes[*commit.Id()]; ok {
		a.When, c.When = date, date
	}
	t, e = commit.Tree()
	return
}

// Historiography struct is responsible of creating and deleting a temporary
// branch to perform commits change. It holds a reference of the HEAD branch
// which will be overriden if a call to Override() is performed.
type Historiography struct {
	repo       *git.Repository
	head       *git.Reference
	tmp        *git.Reference
	checkout   git.CheckoutOpts
	cherrypick git.CherrypickOptions
}

func (h *Historiography) Override() error {
	commit, err := h.repo.LookupCommit(h.tmp.Target())
	if err != nil {
		return err
	}
	branch, err := h.head.Branch().Name()
	if err != nil {
		return err
	}
	_, err = h.repo.CreateBranch(branch, commit, true)
	return err
}

func (h *Historiography) Delete() (err error) {
	var ref *git.Reference
	if h.tmp == nil {
		return
	}
	if ref, err = h.tmp.Resolve(); err != nil {
		return nil // branch does not exist anymore, abort
	}
	if err = h.repo.SetHead(h.head.Name()); err != nil {
		return
	}
	if err = h.repo.CheckoutHead(&h.checkout); err != nil {
		return
	}
	if err = ref.Delete(); err != nil {
		return
	}
	return
}

func (h *Historiography) Free() {
	if err := h.repo.StateCleanup(); err != nil {
		glog.Errorf("cleaning repo state failed: %s", err)
	}
	if err := h.Delete(); err != nil {
		glog.Errorf("cleaning repo state failed: %s", err)
	}

	h.tmp.Free()
}

func NewHistoriography(repo *git.Repository) (h *Historiography, err error) {
	h = &Historiography{repo: repo}
	h.checkout = git.CheckoutOpts{Strategy: git.CheckoutForce}

	if repo.State() != git.RepositoryStateNone {
		return nil, fmt.Errorf("repository is not in a clear state")
	}

	if h.cherrypick, err = git.DefaultCherrypickOptions(); err != nil {
		return
	}

	if h.head, err = h.repo.Head(); err != nil {
		return
	}

	if h.tmp, err = tmpBranch(repo); err != nil {
		return
	}

	if err = h.repo.SetHead(h.tmp.Name()); err != nil {
		h.Free()
		return
	}

	if err = h.repo.CheckoutHead(&h.checkout); err != nil {
		h.Free()
		return
	}

	return
}

func (h *Historiography) Apply(commit *git.Commit, changes Changes) error {
	err := h.repo.Cherrypick(commit, h.cherrypick)
	if err != nil {
		return err
	}

	r, m, a, c, t, err := getArgs(h, commit, changes)
	if err != nil {
		return err
	}

	parent, err := h.repo.LookupCommit(h.tmp.Target())
	if err != nil {
		return err
	}

	_, err = h.repo.CreateCommit(r, a, c, m, t, parent)
	if err != nil {
		return err
	}

	return h.UpdateTmpRef()
}

func (h *Historiography) UpdateTmpRef() error {
	ref, err := h.tmp.Resolve()
	if err != nil {
		return err
	}
	h.tmp.Free()
	h.tmp = ref
	return nil
}

func (h *Historiography) Process(commits Commits, changes Changes) (err error) {
	root := commits[0]

	r, m, a, c, t, err := getArgs(h, root, changes)
	if err != nil {
		return
	}

	if _, err = root.Amend(r, a, c, m, t); err != nil {
		return
	}

	if err = h.UpdateTmpRef(); err != nil {
		return
	}

	for _, commit := range commits[1:] {
		if err = h.Apply(commit, changes); err != nil {
			return
		}
	}
	return
}

func (h *Historiography) Confirm(c bool) (err error) {
	if !c {
		c, err = Confirm(h.repo)
	}
	if c {
		err = h.Override()
	}
	return
}
