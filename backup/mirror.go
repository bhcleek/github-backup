package backup

import (
	"errors"
	"net/url"
	"os"
	"os/exec"
)

type Mirror struct {
	path string
}

func NewMirror(path string) *Mirror {
	return &Mirror{path}
}

func (b *Mirror) Backup(remote url.URL, verbose bool) error {
	if b == nil {
		return nil
	}

	// check whether the backup already exists
	if stat, err := os.Stat(b.path); os.IsNotExist(err) {
		err = os.MkdirAll(b.path, 0777)
		if err != nil {
			return errors.New("could not create " + b.path)
		}

		cloneCommand := exec.Command("git", "clone", "--mirror", remote.String(), b.path)
		if verbose {
			cloneCommand.Stderr = os.Stderr
		}
		err = cloneCommand.Run()
		if err != nil {
			return err
		}
	} else {
		if !stat.IsDir() {
			return errors.New(b.path + " exists, but is a file")
		}
	}

	fetchCommand := exec.Command("git", "fetch", "--prune", "origin")
	fetchCommand.Dir = b.path
	if verbose {
		fetchCommand.Stderr = os.Stderr
	}
	err := fetchCommand.Run()
	if err != nil {
		return err
	}
	return nil
}
