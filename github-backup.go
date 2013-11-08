package main

import (
	"code.google.com/p/goauth2/oauth"
	"flag"
	"fmt"
	"github.com/bhcleek/github-backup/backup"
	"github.com/google/go-github/github"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"
)

var (
	cacheFile   = flag.String("cache", "", "The access token cache file.")
	accessToken = flag.String("token", "", "The OAuth access token.")
	backupDir   = flag.String("to", ".", "The base directory for repository backups.")
)

func init() {
	const usageMsg = `
DESCRIPTION
	Backup GitHub repositories. All access to GitHub will use OAuth tokens. username/password authentication is not supported.

OPTIONS
	-token TOKEN
	use TOKEN for the token instead of the value in the token cache file.
	-cache FILE
	if given a token (-token TOKEN), write its value into FILE. When -token is not used, read the token to use from FILE.
`

	flag.Parse()

	if *accessToken == "" && *cacheFile == "" {
		flag.Usage()
		fmt.Fprintf(os.Stderr, usageMsg)
		os.Exit(2)
	}

	if *backupDir == flag.Lookup("to").DefValue {
		wd, err := os.Getwd()

		if err != nil {
			log.Fatal(err)
		}

		*backupDir = wd
	}
}

func main() {
	var (
		err       error
		token     *oauth.Token
		transport *oauth.Transport
		cache     oauth.Cache
		wg        sync.WaitGroup
	)

	config := &oauth.Config{}

	if *accessToken == "" {
		cache = oauth.CacheFile(*cacheFile)

		token, err = cache.Token()

		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Token is cached in %v\n", cache)
	} else {
		token = &oauth.Token{AccessToken: *accessToken}

		if *cacheFile != "" {
			cache = oauth.CacheFile(*cacheFile)
			cache.PutToken(token)
		}
	}

	transport = &oauth.Transport{Config: config}
	transport.Token = token

	client := github.NewClient(transport.Client())

	user, _, err := client.Users.Get("")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Retrieving information from GitHub using credentials of", *user.Login)

	opt := &github.RepositoryListOptions{Type: "all"}
	repos, _, err := client.Repositories.List("", opt)

	if err != nil {
		log.Fatal(err)
	}

	if len(repos) == 0 {
		fmt.Println("no user repositories available.")
	}

	orgs, _, err := client.Organizations.List("", &github.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}

	for _, org := range orgs {
		opt := &github.RepositoryListByOrgOptions{Type: "all"}
		orgRepos, _, err := client.Repositories.ListByOrg(*org.Login, opt)

		if err != nil {
			log.Fatal(err)
		}

		if len(orgRepos) == 0 {
			fmt.Println("no", *org.Login, "repositories available")
		} else {
			repos = append(repos, orgRepos...)
		}
	}

	configCredential := exec.Command("git", "config", "--global", "credential.helper", "cache")
	err = configCredential.Run()
	if err != nil {
		log.Fatal(err)
	}

	gitCredential := exec.Command("git", "credential", "approve")
	gitCredential.Stdin = strings.NewReader(fmt.Sprintf("protocol=https\nhost=github.com\nusername=%s\npassword=%s\n\n", *user.Login, token.AccessToken))
	err = gitCredential.Run()
	if err != nil {
		log.Fatal(err)
	}

	for _, repo := range repos {
		wg.Add(1)
		go func(repo github.Repository) {
			remote, err := url.Parse(*repo.CloneURL)
			if err != nil {
				fmt.Println(err)
			} else {
				mirrorPathSegments := make([]string, 0, 4)
				mirrorPathSegments = append(mirrorPathSegments, *backupDir)
				host := strings.Split(remote.Host, ":")[0] // strip off the port portion if it's there.
				mirrorPathSegments = append(mirrorPathSegments, host)
				mirrorPathSegments = append(mirrorPathSegments, strings.Split(remote.Path, "/")...)
				mirrorPath := path.Join(mirrorPathSegments...)

				mirror := backup.NewMirror(mirrorPath)
				err = mirror.Backup(*remote)
				if err != nil {
					fmt.Println(err)
				}
			}
			wg.Done()
		}(repo)
	}

	done := make(chan int)
	go func() {
		wg.Wait()
		done <- 1
	}()

	for {
		select {
		case <-time.After(2 * time.Second):
			fmt.Printf(".")
		case <-done:
			return
		}
	}
}
