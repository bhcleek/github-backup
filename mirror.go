package main

import (
	"errors"
	"net/url"
	"os"

	"github.com/libgit2/git2go"
)

type Mirror struct {
	path                string
	remote              url.URL
	credentialsCallback git.CredentialsCallback
}

func NewMirror(path string, remote url.URL, credentialsCallback git.CredentialsCallback) *Mirror {
	return &Mirror{
		path,
		remote,
		credentialsCallback,
	}
}

func (b *Mirror) Fetch() error {
	if b == nil {
		return nil
	}

	// check whether the backup already exists
	if stat, err := os.Stat(b.path); os.IsNotExist(err) {
		err = os.MkdirAll(b.path, 0777)
		if err != nil {
			return errors.New("could not create " + b.path)
		}

		opt := &git.CloneOptions{
			RemoteCallbacks: &git.RemoteCallbacks{
				CredentialsCallback: b.credentialsCallback,
			},
			Bare: true,
		}
		_, err := git.Clone(b.remote.String(), b.path, opt)
		if err != nil {
			return err
		}
	} else {
		if !stat.IsDir() {
			return errors.New(b.path + " exists, but is a file")
		}
	}

	repo, err := git.OpenRepository(b.path)
	if err != nil {
		return err
	}

	remote, err := repo.LoadRemote("origin")
	if err != nil {
		return err
	}
	err = remote.Fetch(nil, "")
	return err
}
