package historiography

import (
	"fmt"
	git "gopkg.in/libgit2/git2go.v26"
	"os"
	"os/exec"
	"strings"
)

// Helper function for displaying git log after rescheduling and ask user for
// input.
// It checks if git is available in path, then run a git log. After review is
// done it asks for validation/redisplay/cancellation.
func Confirm(repo *git.Repository) (ok bool, err error) {

	var response, path string

	path, err = exec.LookPath("git")
	if err != nil {
		fmt.Fprintf(os.Stderr, "git not found in path, can not display logs")
		return
	}

	cmd := &exec.Cmd{
		Path: path, Args: []string{"git", "log"}, Dir: repo.Path(),
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
	}

	err = cmd.Run()
	if err != nil {
		return
	}
	for {

		fmt.Println("Is rescheduling correct? [Y/n] (see again? [?]): ")
		_, err = fmt.Scanln(&response)
		if err != nil {
			return true, nil // default choice
		}
		response = strings.ToLower(response[:1])
		switch response {
		case "y":
			return true, err
		case "n":
			return
		case "?":
			return Confirm(repo)
		default:
			fmt.Fprintf(os.Stderr, "\nResponse is incorrect!\n")
		}
	}
}
