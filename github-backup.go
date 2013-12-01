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

var VERSION = "dev" // set correctly by the linker (e.g. go build -ldflags "-X main.VERSION <semver>")

var (
	cacheFile   = flag.String("cache", "", "The access token cache file.")
	accessToken = flag.String("token", "", "The OAuth access token.")
	backupDir   = flag.String("to", ".", "The base directory for repository backups.")
	verbose     = flag.Bool("verbose", false, "Be verbose.")
	showVersion = flag.Bool("version", false, "Print version and exit")
	showHelp    = flag.Bool("help", false, "Print usage and exit")
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

func seedGitCredentials(user string, pass string) {
	configCredential := exec.Command("git", "config", "--global", "credential.helper", "cache")
	err := configCredential.Run()
	if err != nil {
		log.Fatal(err)
	}

	gitCredential := exec.Command("git", "credential", "approve")
	gitCredential.Stdin = strings.NewReader(fmt.Sprintf("protocol=https\nhost=github.com\nusername=%s\npassword=%s\n\n", user, pass))
	err = gitCredential.Run()
	if err != nil {
		log.Fatal(err)
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

				mirror := backup.NewMirror(mirrorPath)
				err = mirror.Backup(*remote)
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

	seedGitCredentials(*user.Login, token.AccessToken)

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
