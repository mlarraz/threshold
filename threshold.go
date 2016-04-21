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

	defer handleResponse(w, &res, &code)

	err := json.NewDecoder(r.Body).Decode(&webhook)
	if err != nil {
		res = fmt.Sprintf("Problem decoding webhook payload: %s", err)
		log.Println(res)
		code = http.StatusBadRequest

		return
	}

	// If webhook is empty, formatting is not correct
	if (webhook == github.PullRequestEvent{}) {
		res = "Event is not a pull request. Ignoring"
		log.Println(res)
		code = http.StatusOK
		return
	}

	if webhook.Action == nil || *webhook.Action == "closed" {
		res = "Invalid PR action. Ignoring."
		log.Println(res)
		code = http.StatusOK

		return
	}

	pr := webhook.PullRequest
	errs := Evaluate(pr)

	owner := webhook.Repo.Owner.Login
	repo := webhook.Repo.Name
	num := webhook.Number

	if len(errs) == 0 {
		// Add a passing status
		status, err := CreateStatus(pr, "success")
		if err != nil {
			res = fmt.Sprintf("%s", err)
			log.Println(res)

			code = http.StatusInternalServerError
		} else {
			res = fmt.Sprintf("Successfully posted status at: %s", *status.URL)
			log.Println(res)

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
		log.Println(res)
		code = http.StatusInternalServerError

		return
	}

	if Strict {
		// Close the PR
		*pr.State = "closed"
		_, _, err = Client.PullRequests.Edit(*owner, *repo, *num, pr)
		if err != nil {
			res = fmt.Sprintf("Error closing PR: %s", err)
			log.Println(res)

			code = http.StatusInternalServerError
		} else {
			res = fmt.Sprintf("Closed PR at: %s", *pr.URL)
			log.Println(res)

			code = http.StatusOK
		}
	} else {
		// Add a failing status
		status, err := CreateStatus(pr, "failure")
		if err != nil {
			res = fmt.Sprintf("Error updating status: %s", err)
			log.Println(res)

			code = http.StatusInternalServerError
		} else {
			res = fmt.Sprintf("Successfully posted status at: %s", *status.URL)
			log.Println(res)

			code = http.StatusOK
		}
	}
}

func handleResponse(w http.ResponseWriter, body *string, code *int) {
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

	desc := "Complexity thresholds"
	context := "ci/threshold"

	status := &github.RepoStatus{
		State:       &s,
		Description: &desc,
		Context:     &context,
	}

	owner := pr.Base.User.Login
	repo := pr.Base.Repo.Name

	res, _, err := Client.Repositories.CreateStatus(*owner, *repo, *pr.Head.SHA, status)
	return res, err
}

func Evaluate(pr *github.PullRequest) (errs []string) {
	if MaxFiles != 0 && *pr.ChangedFiles > MaxFiles {
		msg := fmt.Sprintf("* %d files were changed, but the threshold is %d\n", *pr.ChangedFiles, MaxFiles)
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
