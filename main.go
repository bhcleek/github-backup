package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"code.google.com/p/goauth2/oauth"
	"github.com/google/go-github/github"
	"github.com/libgit2/git2go"
)

var VERSION = "dev" // set correctly by the linker (e.g. go build -ldflags "-X main.VERSION <semver>")

var (
	cacheFile           = flag.String("cache", "", "The access token cache file.")
	accessToken         = flag.String("token", "", "The OAuth access token.")
	backupDir           = flag.String("to", ".", "The base directory for repository backups.")
	verbose             = flag.Bool("verbose", false, "Be verbose.")
	showVersion         = flag.Bool("version", false, "Print version and exit")
	showHelp            = flag.Bool("help", false, "Print usage and exit")
	credentialsCallback git.CredentialsCallback
)

func init() {
	const usageMsg = `
DESCRIPTION
	Backup GitHub repositories. All access to GitHub will use OAuth tokens. username/password authentication is not supported.

OPTIONS
	-help
	Print usage and exit

	-token TOKEN
	use TOKEN for the token instead of the value in the token cache file.

	-cache FILE
	if given a token (-token TOKEN), write its value into FILE. When -token is not used, read the token to use from FILE.

	-to DIR
	use DIR as the base directory for backups. Defaults to the current directory.

	-verbose
	Be verbose: log results for each repository.

	-version
	Print version and exit.
`

	flag.Parse()

	if *showHelp {
		fmt.Print(usageMsg)
		os.Exit(0)
	}

	if *showVersion {
		fmt.Println(VERSION)
		os.Exit(0)
	}

	if *accessToken == "" && *cacheFile == "" {
		flag.Usage()
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

func feedRepositoryQueue(client *github.Client, queue chan github.Repository, log chan string) {
	defer close(queue)

	opt := &github.RepositoryListOptions{}

	for {
		repos, resp, err := client.Repositories.List("", opt)

		if err != nil {
			log <- err.Error()
			break
		}

		if opt.Page == 0 && len(repos) == 0 {
			log <- "No user repositories available"
			break
		} else {
			for _, repo := range repos {
				queue <- repo
			}
			if resp.NextPage != 0 {
				opt.Page = resp.NextPage
			} else {
				break
			}
		}
	}

	orgs, _, err := client.Organizations.List("", &github.ListOptions{})
	if err != nil {
		log <- err.Error()
	}

	for _, org := range orgs {
		opt := &github.RepositoryListByOrgOptions{Type: "all"}

		for {
			repos, resp, err := client.Repositories.ListByOrg(*org.Login, opt)

			if err != nil {
				log <- err.Error()
				break
			}

			if opt.Page == 0 && len(repos) == 0 {
				log <- fmt.Sprintf("no %s repositories available", *org.Login)
				break
			} else {
				for _, repo := range repos {
					queue <- repo
				}
				if resp.NextPage != 0 {
					opt.Page = resp.NextPage
				} else {
					break
				}
			}
		}
	}
}

func processQueue(queue chan github.Repository, verboseLog chan string, done chan int) {
	wg := sync.WaitGroup{}

	for repo := range queue {
		wg.Add(1)
		go func(repo github.Repository) {
			remote, err := url.Parse(*repo.CloneURL)
			if err != nil {
				log.Println(*repo.Name, err)
			} else {
				verboseLog <- fmt.Sprintf("checking %s", remote.Path[1:])

				mirrorPathSegments := make([]string, 0, 4)
				mirrorPathSegments = append(mirrorPathSegments, *backupDir)
				host := strings.Split(remote.Host, ":")[0] // strip off the port portion if it's there.
				mirrorPathSegments = append(mirrorPathSegments, host)
				mirrorPathSegments = append(mirrorPathSegments, strings.Split(remote.Path, "/")...)
				mirrorPath := path.Join(mirrorPathSegments...)

				mirror := NewMirror(mirrorPath, *remote, credentialsCallback)
				err = mirror.Fetch()

				if err != nil {
					log.Println(remote.Path, err)
				} else {
					verboseLog <- fmt.Sprintf("%s complete", remote.Path[1:])
				}
			}
			wg.Done()
		}(repo)
	}

	wg.Wait()
	done <- 1
}

func main() {
	var (
		err       error
		token     *oauth.Token
		transport *oauth.Transport
		cache     oauth.Cache
	)

	config := &oauth.Config{}

	if *accessToken == "" {
		cache = oauth.CacheFile(*cacheFile)

		token, err = cache.Token()

		if err != nil {
			log.Fatal(err)
		}

		if *verbose {
			log.Println("Token is cached in", cache)
		}
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

	if *verbose {
		log.Println("Retrieving information from GitHub using credentials of", *user.Login)
	}

	credentialsCallback = func(url string, username_from_url string, allowed_type git.CredType) (int, *git.Cred) {
		log.Println(username_from_url)
		i, c := git.NewCredUserpassPlaintext(*user.Login, token.AccessToken)
		return i, &c
	}

	msgQueue := make(chan string)
	queue := make(chan github.Repository)
	done := make(chan int)

	go func(verbose bool, c chan string) {
		select {
		case msg := <-c:
			if verbose {
				log.Println(msg)
			}
		}
	}(*verbose, msgQueue)

	go feedRepositoryQueue(client, queue, msgQueue)
	go processQueue(queue, msgQueue, done)

	for {
		select {
		case <-time.After(2 * time.Second):
			if !*verbose {
				os.Stderr.WriteString(".")
			}
		case msg := <-msgQueue:
			if *verbose {
				log.Println(msg)
			}
		case <-done:
			return
		}
	}
}
