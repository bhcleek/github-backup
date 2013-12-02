# github-backup

github-backup creates mirrors of all of a user's repositories and all repositories to which a user has access via team membership in an organization.

# help

documentation for all flags can be seen with 

    github-backup -help

## first use

[Create an access token](https://help.github.com/articles/creating-an-access-token-for-command-line-use) for GitHub and then run

    github-backup -token <ACCESS_TOKEN> -cache /path/to/cache/file -to /mirror/base/path

## use a cached token 

github-backup will use the cached token (`-cache cache.json`) when run without the `-token` flag.

    github-backup -cache /path/to/cache/file -to /mirror/base/path

# Credits
* (go-github)[http://github.com/google/go-github] - License: (BSD 3-Clause)[https://github.com/google/go-github/blob/master/LICENSE] go-github provides a library to access the GitHub API
* (goauth2)[http://code.google.com/p/goauth2] - License: (BSD 3-Clause)[https://code.google.com/p/goauth2/source/browse/LICENSE] goauth2 provides a library for OAuth 2.0.
* (go-querystring)[http://github.com/google/go-querystring] - License: (BSD 3-Clause)[https://github.com/google/go-querystring/blob/master/LICENSE]

