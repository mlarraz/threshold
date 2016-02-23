# Threshold

Set complexity thresholds to automatically triage PRs to your Github projects.

## Usage

Threshold is meant for use in a web server that listens for Github webhooks.
It ships a default handler for this, so a minimal implementation could be:

```go
package main

import (
    "log"
    "net/http"

    "github.com/mlarraz/threshold"
)

func main() {
    // Sets default github client. You can use your own if necessary.
    threshold.Client = threshold.CreateClient()

    http.HandleFunc("/", threshold.Handler)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

That's pretty boring, though. You want to set some rules:

```go
threshold.MaxCommits  = 20
threshold.MaxComments = 50
threshold.MaxLines    = 1000
threshold.MaxFiles    = 30
```

Now if a PR exceeds any of these, a comment will get posted, and a failing status will be applied to it (just like a CI failure).

### Strict Mode

If you set
```go
threshold.Strict = true
```
then failing PRs will automatically be closed.

## Configuration
The default Github client will expect an auth token:

```go
threshold.Token = "myauthtoken"
```

You can also use a different Github installation:
```go
threshold.Host = "https://git.mycompany.com/api/v3/"
```

