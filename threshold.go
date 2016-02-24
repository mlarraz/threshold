package threshold

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var (
	Host   string
	Token  string
	Client *github.Client

	// TODO: Make these not be global
	MaxCommits  int
	MaxComments int
	MaxLines    int
	MaxFiles    int

	Strict bool
)

func Handler(w http.ResponseWriter, r *http.Request) {
	var webhook github.PullRequestEvent
	var res string
	var code int

	defer HandleResponse(w, &res, &code)

	err := json.NewDecoder(r.Body).Decode(&webhook)
	if err != nil {
		res = fmt.Sprintf("Problem decoding webhook payload: %s", err)
		code = http.StatusBadRequest

		return
	}

	if *webhook.Action == "closed" {
		res = "Invalid PR action. Ignoring."
		code = http.StatusOK

		return
	}

	pr := webhook.PullRequest
	errs := Evaluate(pr)

	owner := webhook.Repo.Owner.Name
	repo := webhook.Repo.Name
	num := webhook.Number

	if len(errs) == 0 {
		// Add a passing status
		status, err := CreateStatus(pr, "success")
		if err != nil {
			res = fmt.Sprintf("%s", err)
			code = http.StatusInternalServerError
		} else {
			res = fmt.Sprintf("Successfully posted status at: %s", *status.URL)
			code = http.StatusOK
		}

		return
	}

	b := "This PR has been judged to be too complex for the following reasons:\n\n" +
		strings.Join(errs, "\n") +
		"\nPlease consider breaking these changes up in to smaller pieces."

	comment := github.IssueComment{Body: &b}

	// TODO: Check if we've already posted a comment
	// comments, _, err := Client.Issues.ListComments(*owner, *repo, *num, &github.IssueListCommentsOptions{})

	// Post a comment
	_, _, err = Client.Issues.CreateComment(*owner, *repo, *num, &comment)
	if err != nil {
		res = fmt.Sprintf("Error posting a comment: %s", err)
		code = http.StatusInternalServerError

		return
	}

	if Strict {
		// Close the PR
		*pr.State = "closed"
		_, _, err = Client.PullRequests.Edit(*owner, *repo, *num, pr)
		if err != nil {
			res = fmt.Sprintf("Error closing PR: %s", err)
			code = http.StatusInternalServerError
		} else {
			res = fmt.Sprintf("Closed PR at: %s", *pr.URL)
			code = http.StatusOK
		}
	} else {
		// Add a failing status
		status, err := CreateStatus(pr, "failure")
		if err != nil {
			res = fmt.Sprintf("Error updating status: %s", err)
			code = http.StatusInternalServerError
		} else {
			res = fmt.Sprintf("Successfully posted status at: %s", *status.URL)
			code = http.StatusOK
		}
	}
}

func HandleResponse(w http.ResponseWriter, body *string, code *int) {
	log.Println(*body)

	w.Write([]byte(*body))
	w.WriteHeader(*code)
}

func CreateStatus(pr *github.PullRequest, s string) (*github.RepoStatus, error) {
	// Handle invalid state
	if s != "failure" && s != "success" {
		err := fmt.Errorf("Invalid state: %s. Valid states are \"failure\" and \"success\"", s)
		return nil, err
	}

	status := github.RepoStatus{}
	*status.State = s
	*status.Description = "Complexity thresholds"
	*status.Context = "ci/threshold"

	owner := pr.Base.User.Name
	repo := pr.Base.Repo.Name

	res, _, err := Client.Repositories.CreateStatus(*owner, *repo, *pr.Head.SHA, &status)
	return res, err
}

func Evaluate(pr *github.PullRequest) (errs []string) {
	if MaxFiles != 0 && *pr.ChangedFiles > MaxFiles {
		msg := fmt.Sprintf("%d files were changed, but the threshold is %d", *pr.ChangedFiles, MaxFiles)
		errs = append(errs, msg)
	}

	return errs
}

func CreateClient() *github.Client {
	var tc *http.Client

	if len(Token) != 0 {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: Token},
		)
		tc = oauth2.NewClient(oauth2.NoContext, ts)
	}

	client := github.NewClient(tc)

	if len(Host) != 0 {
		baseURL, err := url.Parse(Host)

		if err != nil {
			log.Fatalf("Error parsing host %s: %s", Host, err)
		}

		client.BaseURL = baseURL
	}

	return client
}
