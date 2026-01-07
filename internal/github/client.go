package github

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client
type Client struct {
	client *github.Client
	ctx    context.Context
}

// PRReference holds parsed PR information
type PRReference struct {
	Owner  string
	Repo   string
	Number int
}

// FileChange represents a changed file in a PR
type FileChange struct {
	Filename    string
	Status      string // added, modified, removed, renamed
	Additions   int
	Deletions   int
	Patch       string // The diff patch
	PreviousName string // For renamed files
}

// ReviewComment represents a comment to be posted
type ReviewComment struct {
	Path     string
	Line     int
	Body     string
	Side     string // LEFT or RIGHT
}

// PRComment represents an existing comment on a PR
type PRComment struct {
	ID        int64
	User      string
	Body      string
	Path      string
	Line      int
	CreatedAt string
	InReplyTo int64
}

// NewClient creates a new GitHub client with the given token
func NewClient(token string) *Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return &Client{
		client: github.NewClient(tc),
		ctx:    ctx,
	}
}

// ParsePRReference parses various PR reference formats
// Supports: owner/repo#123, https://github.com/owner/repo/pull/123
func ParsePRReference(ref string) (*PRReference, error) {
	// Try URL format first
	urlPattern := regexp.MustCompile(`github\.com/([^/]+)/([^/]+)/pull/(\d+)`)
	if matches := urlPattern.FindStringSubmatch(ref); matches != nil {
		num, _ := strconv.Atoi(matches[3])
		return &PRReference{
			Owner:  matches[1],
			Repo:   matches[2],
			Number: num,
		}, nil
	}

	// Try owner/repo#number format
	shortPattern := regexp.MustCompile(`^([^/]+)/([^#]+)#(\d+)$`)
	if matches := shortPattern.FindStringSubmatch(ref); matches != nil {
		num, _ := strconv.Atoi(matches[3])
		return &PRReference{
			Owner:  matches[1],
			Repo:   matches[2],
			Number: num,
		}, nil
	}

	return nil, fmt.Errorf("invalid PR reference format: %s (use owner/repo#123 or GitHub URL)", ref)
}

// GetPR fetches PR details
func (c *Client) GetPR(ref *PRReference) (*github.PullRequest, error) {
	pr, _, err := c.client.PullRequests.Get(c.ctx, ref.Owner, ref.Repo, ref.Number)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR: %w", err)
	}
	return pr, nil
}

// GetPRFiles returns the list of changed files in a PR
func (c *Client) GetPRFiles(ref *PRReference) ([]*FileChange, error) {
	opts := &github.ListOptions{PerPage: 100}
	var allFiles []*FileChange

	for {
		files, resp, err := c.client.PullRequests.ListFiles(c.ctx, ref.Owner, ref.Repo, ref.Number, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch PR files: %w", err)
		}

		for _, f := range files {
			fc := &FileChange{
				Filename:  f.GetFilename(),
				Status:    f.GetStatus(),
				Additions: f.GetAdditions(),
				Deletions: f.GetDeletions(),
				Patch:     f.GetPatch(),
			}
			if f.GetStatus() == "renamed" {
				fc.PreviousName = f.GetPreviousFilename()
			}
			allFiles = append(allFiles, fc)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allFiles, nil
}

// GetFileContent fetches the content of a file at a specific ref
func (c *Client) GetFileContent(owner, repo, path, ref string) (string, error) {
	content, _, _, err := c.client.Repositories.GetContents(c.ctx, owner, repo, path, &github.RepositoryContentGetOptions{
		Ref: ref,
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch file content: %w", err)
	}

	decoded, err := content.GetContent()
	if err != nil {
		return "", fmt.Errorf("failed to decode file content: %w", err)
	}

	return decoded, nil
}

// GetRelatedFiles finds files that might be related (imports, tests, etc.)
func (c *Client) GetRelatedFiles(owner, repo, path, ref string) ([]string, error) {
	var related []string

	// Get the directory
	dir := getDirectory(path)
	filename := getFilename(path)
	ext := getExtension(path)
	baseName := strings.TrimSuffix(filename, ext)

	// Look for test files
	testPatterns := []string{
		dir + "/" + baseName + "_test" + ext,
		dir + "/" + baseName + ".test" + ext,
		dir + "/" + baseName + ".spec" + ext,
		"test/" + path,
		"tests/" + path,
	}

	for _, pattern := range testPatterns {
		if _, err := c.GetFileContent(owner, repo, pattern, ref); err == nil {
			related = append(related, pattern)
		}
	}

	return related, nil
}

// GetPRComments fetches all review comments on a PR
func (c *Client) GetPRComments(ref *PRReference) ([]*PRComment, error) {
	opts := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	var allComments []*PRComment

	for {
		comments, resp, err := c.client.PullRequests.ListComments(c.ctx, ref.Owner, ref.Repo, ref.Number, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch PR comments: %w", err)
		}

		for _, c := range comments {
			pc := &PRComment{
				ID:        c.GetID(),
				User:      c.GetUser().GetLogin(),
				Body:      c.GetBody(),
				Path:      c.GetPath(),
				Line:      c.GetLine(),
				CreatedAt: c.GetCreatedAt().String(),
				InReplyTo: c.GetInReplyTo(),
			}
			allComments = append(allComments, pc)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allComments, nil
}

// PostReview submits a review with comments
func (c *Client) PostReview(ref *PRReference, body string, event string, comments []*ReviewComment) error {
	var ghComments []*github.DraftReviewComment
	for _, rc := range comments {
		ghComments = append(ghComments, &github.DraftReviewComment{
			Path: github.String(rc.Path),
			Line: github.Int(rc.Line),
			Body: github.String(rc.Body),
			Side: github.String(rc.Side),
		})
	}

	review := &github.PullRequestReviewRequest{
		Body:     github.String(body),
		Event:    github.String(event), // APPROVE, REQUEST_CHANGES, COMMENT
		Comments: ghComments,
	}

	_, _, err := c.client.PullRequests.CreateReview(c.ctx, ref.Owner, ref.Repo, ref.Number, review)
	if err != nil {
		return fmt.Errorf("failed to post review: %w", err)
	}

	return nil
}

// ReplyToComment posts a reply to an existing comment
func (c *Client) ReplyToComment(ref *PRReference, commentID int64, body string) error {
	_, _, err := c.client.PullRequests.CreateCommentInReplyTo(c.ctx, ref.Owner, ref.Repo, ref.Number, body, commentID)
	if err != nil {
		return fmt.Errorf("failed to reply to comment: %w", err)
	}
	return nil
}

// Helper functions
func getDirectory(path string) string {
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash == -1 {
		return "."
	}
	return path[:lastSlash]
}

func getFilename(path string) string {
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash == -1 {
		return path
	}
	return path[lastSlash+1:]
}

func getExtension(path string) string {
	filename := getFilename(path)
	lastDot := strings.LastIndex(filename, ".")
	if lastDot == -1 {
		return ""
	}
	return filename[lastDot:]
}
