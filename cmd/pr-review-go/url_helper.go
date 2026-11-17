package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	githubURLRegex        = regexp.MustCompile(`/([^/]+)/([^/]+)/pull/(\d+)`)
	bitbucketCloudRegex   = regexp.MustCompile(`/([^/]+)/([^/]+)/pull-requests/(\d+)`)
	bitbucketServerRegex  = regexp.MustCompile(`/projects/([^/]+)/repos/([^/]+)/pull-requests/(\d+)`)
)

type parsedPR struct {
	Provider string
	Owner    string
	Repo     string
	Number   int
	BaseURL  string
}

func parsePRURL(rawURL string) (*parsedPR, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url %q: %w", rawURL, err)
	}

	path := u.Path

	// 1. Bitbucket Server pattern: /projects/{proj}/repos/{repo}/pull-requests/{id}
	if matches := bitbucketServerRegex.FindStringSubmatch(path); len(matches) == 4 {
		num, _ := strconv.Atoi(matches[3])
		baseURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		return &parsedPR{
			Provider: "bitbucket_server",
			Owner:    matches[1],
			Repo:     matches[2],
			Number:   num,
			BaseURL:  baseURL,
		}, nil
	}

	// 2. Bitbucket Cloud pattern: /workspace/repo/pull-requests/{id}
	if strings.Contains(u.Host, "bitbucket.org") {
		if matches := bitbucketCloudRegex.FindStringSubmatch(path); len(matches) == 4 {
			num, _ := strconv.Atoi(matches[3])
			return &parsedPR{
				Provider: "bitbucket_cloud",
				Owner:    matches[1],
				Repo:     matches[2],
				Number:   num,
			}, nil
		}
	}

	// 3. GitHub pattern: /{owner}/{repo}/pull/{number}
	if matches := githubURLRegex.FindStringSubmatch(path); len(matches) == 4 {
		num, _ := strconv.Atoi(matches[3])
		prov := "github"
		baseURL := ""
		if !strings.Contains(u.Host, "github.com") {
			// GitHub Enterprise Server
			baseURL = fmt.Sprintf("%s://%s/api/v3", u.Scheme, u.Host)
		}
		return &parsedPR{
			Provider: prov,
			Owner:    matches[1],
			Repo:     matches[2],
			Number:   num,
			BaseURL:  baseURL,
		}, nil
	}

	return nil, fmt.Errorf("unable to determine git provider and PR number from URL: %s", rawURL)
}
