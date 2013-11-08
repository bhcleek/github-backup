# github-backup

github-backup creates mirrors of all of a user's repositories and all repositories to which a user has access via team membership in an organization.

## first use

[Create an access token](https://help.github.com/articles/creating-an-access-token-for-command-line-use) for GitHub and then run

    github-backup -token <ACCESS_TOKEN> -cache /path/to/cache/file -to /mirror/base/path

## use a cached token 

github-backup will use the cached token (`-cache cache.json`) when run without the `-token` flag.
