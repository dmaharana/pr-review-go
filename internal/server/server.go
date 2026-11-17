package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"pr-review-go/internal/config"
	"pr-review-go/internal/llm"
	"pr-review-go/internal/provider"
	"pr-review-go/internal/provider/bitbucketcloud"
	"pr-review-go/internal/provider/bitbucketserver"
	"pr-review-go/internal/provider/github"
	"pr-review-go/internal/review"
)

// Server coordinates the HTTP webhook daemon.
type Server struct {
	cfg       *config.Config
	pool      *WorkerPool
	debounce  *DebounceRegistry
	llmClient llm.Client
	httpMux   *http.ServeMux
}

// NewServer constructs the HTTP webhook server.
func NewServer(cfg *config.Config, llmClient llm.Client) *Server {
	s := &Server{
		cfg:       cfg,
		pool:      NewWorkerPool(cfg.Server.Workers, 200),
		debounce:  NewDebounceRegistry(),
		llmClient: llmClient,
		httpMux:   http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.httpMux.HandleFunc("/healthz", s.handleHealthz)
	s.httpMux.HandleFunc("/webhooks/github", s.handleGitHubWebhook)
	s.httpMux.HandleFunc("/webhooks/bitbucket", s.handleBitbucketWebhook)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.httpMux.ServeHTTP(w, r)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok\n"))
}

func (s *Server) verifyGitHubSignature(body []byte, signature string) bool {
	if s.cfg.GitHub.WebhookSecret == "" {
		return true // secret check not configured
	}
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	expectedMAC := strings.TrimPrefix(signature, "sha256=")
	mac := hmac.New(sha256.New, []byte(s.cfg.GitHub.WebhookSecret))
	mac.Write(body)
	actualMAC := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expectedMAC), []byte(actualMAC))
}

func (s *Server) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("X-Hub-Signature-256")
	if !s.verifyGitHubSignature(body, sig) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	event := r.Header.Get("X-GitHub-Event")
	switch event {
	case "pull_request":
		var payload struct {
			Action      string `json:"action"`
			Number      int    `json:"number"`
			Repository struct {
				Owner struct {
					Login string `json:"login"`
				} `json:"owner"`
				Name string `json:"name"`
			} `json:"repository"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		action := payload.Action
		if (action == "opened" && s.cfg.Server.AutoReviewOnOpen) ||
			(action == "synchronize" && s.cfg.Server.AutoReviewOnModify) ||
			action == "reopened" {
			owner := payload.Repository.Owner.Login
			repo := payload.Repository.Name
			num := payload.Number

			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("Review job accepted\n"))

			s.pool.Enqueue(func() {
				s.executeGitHubReview(owner, repo, num)
			})
			return
		}

	case "issue_comment":
		var payload struct {
			Action string `json:"action"`
			Issue  struct {
				Number      int  `json:"number"`
				PullRequest *struct {
					URL string `json:"url"`
				} `json:"pull_request"`
			} `json:"issue"`
			Comment struct {
				Body string `json:"body"`
			} `json:"comment"`
			Repository struct {
				Owner struct {
					Login string `json:"login"`
				} `json:"owner"`
				Name string `json:"name"`
			} `json:"repository"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if payload.Issue.PullRequest != nil && s.cfg.Server.CommentTriggersEnabled && payload.Action == "created" {
			commentText := strings.TrimSpace(payload.Comment.Body)
			owner := payload.Repository.Owner.Login
			repo := payload.Repository.Name
			num := payload.Issue.Number

			if strings.HasPrefix(commentText, "/") {
				w.WriteHeader(http.StatusAccepted)
				w.Write([]byte("Comment command accepted\n"))

				s.pool.Enqueue(func() {
					s.executeCommentCommand("github", owner, repo, num, commentText)
				})
				return
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Event ignored\n"))
}

func (s *Server) handleBitbucketWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	eventKey := r.Header.Get("X-Event-Key")

	// Bitbucket Cloud format
	if strings.HasPrefix(eventKey, "pullrequest:") {
		var payload struct {
			PullRequest struct {
				ID int `json:"id"`
			} `json:"pullrequest"`
			Repository struct {
				Workspace struct {
					Slug string `json:"slug"`
				} `json:"workspace"`
				Name string `json:"name"`
			} `json:"repository"`
			Comment *struct {
				Content struct {
					Raw string `json:"raw"`
				} `json:"content"`
			} `json:"comment"`
		}
		if err := json.Unmarshal(body, &payload); err == nil && payload.PullRequest.ID > 0 {
			workspace := payload.Repository.Workspace.Slug
			repo := payload.Repository.Name
			prID := payload.PullRequest.ID

			if eventKey == "pullrequest:created" || eventKey == "pullrequest:updated" {
				w.WriteHeader(http.StatusAccepted)
				w.Write([]byte("Bitbucket Cloud review accepted\n"))
				s.pool.Enqueue(func() {
					s.executeBitbucketCloudReview(workspace, repo, prID)
				})
				return
			} else if eventKey == "pullrequest:comment_created" && payload.Comment != nil {
				text := strings.TrimSpace(payload.Comment.Content.Raw)
				if strings.HasPrefix(text, "/") {
					w.WriteHeader(http.StatusAccepted)
					w.Write([]byte("Bitbucket comment command accepted\n"))
					s.pool.Enqueue(func() {
						s.executeCommentCommand("bitbucket_cloud", workspace, repo, prID, text)
					})
					return
				}
			}
		}
	}

	// Bitbucket Data Center / Server format: "pr:opened", "pr:modified", "pr:comment:added"
	if strings.HasPrefix(eventKey, "pr:") {
		var payload struct {
			PullRequest struct {
				ID          int `json:"id"`
				ToRef struct {
					Repository struct {
						Project struct {
							Key string `json:"key"`
						} `json:"project"`
						Slug string `json:"slug"`
					} `json:"repository"`
				} `json:"toRef"`
			} `json:"pullRequest"`
			Comment *struct {
				Text string `json:"text"`
			} `json:"comment"`
		}
		if err := json.Unmarshal(body, &payload); err == nil && payload.PullRequest.ID > 0 {
			proj := payload.PullRequest.ToRef.Repository.Project.Key
			repo := payload.PullRequest.ToRef.Repository.Slug
			prID := payload.PullRequest.ID

			if eventKey == "pr:opened" || eventKey == "pr:modified" {
				w.WriteHeader(http.StatusAccepted)
				w.Write([]byte("Bitbucket Server review accepted\n"))
				s.pool.Enqueue(func() {
					s.executeBitbucketServerReview(proj, repo, prID)
				})
				return
			} else if eventKey == "pr:comment:added" && payload.Comment != nil {
				text := strings.TrimSpace(payload.Comment.Text)
				if strings.HasPrefix(text, "/") {
					w.WriteHeader(http.StatusAccepted)
					w.Write([]byte("Bitbucket server comment command accepted\n"))
					s.pool.Enqueue(func() {
						s.executeCommentCommand("bitbucket_server", proj, repo, prID, text)
					})
					return
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Bitbucket event ignored\n"))
}

func (s *Server) executeGitHubReview(owner, repo string, number int) {
	ctx, cancel := s.debounce.Start(context.Background(), "github", fmt.Sprintf("%s/%s", owner, repo), strconv.Itoa(number))
	defer cancel()

	prov, err := github.NewProvider(ctx, s.cfg.GitHub)
	if err != nil {
		log.Printf("Error initializing github provider: %v\n", err)
		return
	}

	pr, err := prov.GetPullRequest(ctx, owner, repo, number)
	if err != nil {
		log.Printf("Error fetching github pr: %v\n", err)
		return
	}

	engine := review.NewEngine(s.cfg, s.llmClient, prov)
	if _, err := engine.Review(ctx, pr); err != nil {
		log.Printf("Error reviewing github pr #%d: %v\n", number, err)
	}
}

func (s *Server) executeBitbucketCloudReview(workspace, repo string, prID int) {
	ctx, cancel := s.debounce.Start(context.Background(), "bitbucket_cloud", fmt.Sprintf("%s/%s", workspace, repo), strconv.Itoa(prID))
	defer cancel()

	prov := bitbucketcloud.NewProvider(s.cfg.BitbucketCloud)
	pr, err := prov.GetPullRequest(ctx, workspace, repo, prID)
	if err != nil {
		log.Printf("Error fetching bitbucket cloud pr: %v\n", err)
		return
	}

	engine := review.NewEngine(s.cfg, s.llmClient, prov)
	if _, err := engine.Review(ctx, pr); err != nil {
		log.Printf("Error reviewing bitbucket cloud pr #%d: %v\n", prID, err)
	}
}

func (s *Server) executeBitbucketServerReview(projectKey, repoSlug string, prID int) {
	ctx, cancel := s.debounce.Start(context.Background(), "bitbucket_server", fmt.Sprintf("%s/%s", projectKey, repoSlug), strconv.Itoa(prID))
	defer cancel()

	prov := bitbucketserver.NewProvider(s.cfg.BitbucketServer)
	pr, err := prov.GetPullRequest(ctx, projectKey, repoSlug, prID)
	if err != nil {
		log.Printf("Error fetching bitbucket server pr: %v\n", err)
		return
	}

	engine := review.NewEngine(s.cfg, s.llmClient, prov)
	if _, err := engine.Review(ctx, pr); err != nil {
		log.Printf("Error reviewing bitbucket server pr #%d: %v\n", prID, err)
	}
}

func (s *Server) executeCommentCommand(providerName, owner, repo string, number int, commandText string) {
	ctx := context.Background()
	var prov provider.GitProvider
	var err error

	switch providerName {
	case "github":
		prov, err = github.NewProvider(ctx, s.cfg.GitHub)
	case "bitbucket_cloud":
		prov = bitbucketcloud.NewProvider(s.cfg.BitbucketCloud)
	case "bitbucket_server":
		prov = bitbucketserver.NewProvider(s.cfg.BitbucketServer)
	}
	if err != nil || prov == nil {
		log.Printf("Error initializing provider for command: %v\n", err)
		return
	}

	pr, err := prov.GetPullRequest(ctx, owner, repo, number)
	if err != nil {
		log.Printf("Error getting PR for comment command: %v\n", err)
		return
	}

	engine := review.NewEngine(s.cfg, s.llmClient, prov)

	parts := strings.Fields(commandText)
	if len(parts) == 0 {
		return
	}
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/review":
		_, _ = engine.Review(ctx, pr)
	case "/describe":
		_, _ = engine.Describe(ctx, pr)
	case "/ask":
		question := strings.TrimSpace(strings.TrimPrefix(commandText, parts[0]))
		if question != "" {
			_, _ = engine.Ask(ctx, pr, question)
		}
	case "/generate_labels":
		_, _ = engine.GenerateLabels(ctx, pr)
	default:
		log.Printf("Unknown comment directive: %s\n", cmd)
	}
}
