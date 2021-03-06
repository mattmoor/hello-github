package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

func main() {
	http.HandleFunc("/", Handler)
	http.ListenAndServe(":8080", nil)
}

func Handler(w http.ResponseWriter, r *http.Request) {
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("ERROR: no payload: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// TODO(mattmoor): This should be:
	//     eventType := github.WebHookType(r)
	// https://github.com/knative/eventing-sources/issues/120
	// HACK HACK HACK
	eventType := strings.Split(r.Header.Get("ce-eventtype"), ".")[4]

	event, err := github.ParseWebHook(eventType, payload)
	if err != nil {
		log.Printf("ERROR: unable to parse webhook: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	handleErr := func(event interface{}, err error) {
		if err == nil {
			fmt.Fprintf(w, "Handled %T", event)
			return
		}
		log.Printf("Error handling %T: %v", event, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// The set of events here should line up with what is in
	//   config/one-time/github-source.yaml
	switch event := event.(type) {
	case *github.PullRequestEvent:
		handleErr(event, HandlePullRequest(event))
	case *github.PushEvent:
		handleErr(event, HandlePush(event))
	case *github.PullRequestReviewEvent:
		handleErr(event, HandleOther(event))
	case *github.PullRequestReviewCommentEvent:
		handleErr(event, HandleOther(event))
	case *github.IssueCommentEvent:
		handleErr(event, HandleIssueComment(event))
	default:
		log.Printf("Unrecognized event: %T", event)
		http.Error(w, "Unknown event", http.StatusBadRequest)
		return
	}
}

func HandleIssueComment(ice *github.IssueCommentEvent) error {
	log.Printf("Comment from %s on #%d: %q",
		ice.Sender.GetLogin(),
		ice.Issue.GetNumber(),
		ice.Comment.GetBody())

	// TODO(mattmoor): Is ice.Repo.Owner.Login reliable for organizations, or do we
	// have to parse the FullName?
	//    Owner: mattmoor, Repo: kontext, Fullname: mattmoor/kontext
	// log.Printf("Owner: %s, Repo: %s, Fullname: %s", *ice.Repo.Owner.Login, *ice.Repo.Name,
	// 	*ice.Repo.FullName)

	if strings.Contains(*ice.Comment.Body, "Hello there.") {
		ctx := context.Background()
		ghc := GetClient(ctx)

		msg := fmt.Sprintf("Hello @%s", ice.Sender.GetLogin())

		_, _, err := ghc.Issues.CreateComment(ctx,
			ice.Repo.Owner.GetLogin(), ice.Repo.GetName(), ice.Issue.GetNumber(),
			&github.IssueComment{
				Body: &msg,
			})
		return err
	}

	return nil
}

func HandlePullRequest(pre *github.PullRequestEvent) error {
	log.Printf("PR: %v", pre.GetPullRequest().String())

	// TODO(mattmoor): To respond to code changes, I think the appropriate set of events are:
	// 1. opened
	// 2. reopened
	// 3. synchronized

	// (from https://developer.github.com/v3/activity/events/types/#pullrequestevent)
	// Other events we might see include:
	// * assigned
	// * unassigned
	// * review_requested
	// * review_request_removed
	// * labeled
	// * unlabeled
	// * edited
	// * closed

	ctx := context.Background()
	ghc := GetClient(ctx)

	msg := fmt.Sprintf("PR event: %v", pre.GetAction())
	_, _, err := ghc.Issues.CreateComment(ctx,
		pre.Repo.Owner.GetLogin(), pre.Repo.GetName(), pre.GetNumber(),
		&github.IssueComment{
			Body: &msg,
		})
	return err
}

func HandlePush(pe *github.PushEvent) error {
	log.Printf("Push: %v", pe.String())
	return nil
}

func HandleOther(event interface{}) error {
	log.Printf("TODO %T: %#v", event, event)
	return nil
}

func GetClient(ctx context.Context) *github.Client {
	return github.NewClient(
		oauth2.NewClient(ctx,
			oauth2.StaticTokenSource(
				&oauth2.Token{
					AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN"),
				},
			),
		),
	)
}
