package historiography

import (
	"fmt"
	"github.com/golang/glog"
	git "gopkg.in/libgit2/git2go.v26"
)

const branchNameSize = 8

// Create a temporary branch using a randomly generated string. Returns
// the reference of this branch to caller.
func tmpBranch(repo *git.Repository) (*git.Reference, error) {
	name := func() string { return SecureRandomString(branchNameSize) }
	root, err := RetrieveRoot(repo) // retrieve first commit of HEAD ref, to create branch from
	if err != nil {
		return nil, err
	}

	for { // we loop here just in case a branch with the same name exists
		branch, err := repo.CreateBranch(name(), root, false)
		if err == nil {
			return branch.Reference, nil
		}
	}
	return nil, nil // this will never been reached
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
	processer  Processer
}

// Build a new Historiography struct, create branches, and hold references.
//
// For the moment the object hold reference from head, in the future we'd like
// to configure the branch from which the temporary branch is created and which
// will be overriden.
func NewHistoriography(repo *git.Repository, p Processer) (h *Historiography, err error) {
	h = &Historiography{repo: repo, processer: p}
	h.checkout = git.CheckoutOpts{Strategy: git.CheckoutForce}

	// non-clean repositories can be dangerous to operate, cancel and raise error
	if repo.State() != git.RepositoryStateNone {
		return nil, fmt.Errorf("repository is not in a clear state")
	}

	if h.cherrypick, err = git.DefaultCherrypickOptions(); err != nil {
		return
	}

	// save ref of HEAD
	if h.head, err = h.repo.Head(); err != nil {
		return
	}

	// create tmp branch
	if h.tmp, err = tmpBranch(repo); err != nil {
		return
	}

	// switch to tmp branch, free the struct in case of an error
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

// Override the saved reference with the temporary branch.
func (h *Historiography) Override() error {

	// retrieve last commit from tmp branch
	commit, err := h.repo.LookupCommit(h.tmp.Target())
	if err != nil {
		return err
	}

	// get saved reference branch name
	branch, err := h.head.Branch().Name()
	if err != nil {
		return err
	}

	// override branch by forcing creation of a new branch with same name
	_, err = h.repo.CreateBranch(branch, commit, true)
	return err
}

// Delete tmp branch if still present and checkout saved ref.
func (h *Historiography) del() (err error) {
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

// Free resources from libgit. Clean repository by deleting tmp branch.
// Checkout saved reference to leave repository in same state as entered.
func (h *Historiography) Free() {
	if err := h.repo.StateCleanup(); err != nil {
		glog.Errorf("cleaning repo state failed: %s", err)
	}
	if err := h.del(); err != nil {
		glog.Errorf("cleaning repo state failed: %s", err)
	}

	h.tmp.Free()
}

// Apply commit on top of the tmp branch, if commit appear in changes, date
// will be updated.
func (h *Historiography) Apply(commit *git.Commit) error {
	err := h.repo.Cherrypick(commit, h.cherrypick)
	if err != nil {
		return err
	}

	r, m, a, c, t, err := h.getArgs(commit)
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

	return h.updateTmpRef()
}

// Each time a commit is applied on tmp branch we have to update our internal
// reference.
func (h *Historiography) updateTmpRef() error {
	ref, err := h.tmp.Resolve()
	if err != nil {
		return err
	}
	h.tmp.Free()
	h.tmp = ref
	return nil
}

// Use embedded processer preprocess function on each day with a list of commits.
func (h *Historiography) Preprocess(commits []Commits) error {
	for _, commit := range commits {
		if err := h.processer.Preprocess(commit); err != nil {
			return err
		}
	}
	return nil
}

// Utilitary function which returns well formated arguments for creating commits.
func (h *Historiography) getArgs(commit *git.Commit) (
	r, m string, a, c *git.Signature, t *git.Tree, e error,
) {
	// retrieve informations from old commit.
	r = h.tmp.Name()

	a, c, m, e = h.processer.Process(commit)
	if e != nil {
		return
	}

	t, e = commit.Tree()
	return
}

// Play commits on top of the temporary branch, embedded processer is called in
// order to furnish informations for the new commit.
func (h *Historiography) Process(commits Commits) (err error) {
	root := commits[0]

	r, m, a, c, t, err := h.getArgs(root)
	if err != nil {
		return
	}

	if _, err = root.Amend(r, a, c, m, t); err != nil {
		return
	}

	if err = h.updateTmpRef(); err != nil {
		return
	}

	for _, commit := range commits[1:] {
		if err = h.Apply(commit); err != nil {
			return
		}
	}
	return
}
