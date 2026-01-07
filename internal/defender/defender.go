package defender

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/user/salty-reviewer/internal/ai"
	"github.com/user/salty-reviewer/internal/config"
	"github.com/user/salty-reviewer/internal/github"
)

// DefenseResult is the output of defending a PR
type DefenseResult struct {
	Responses []CommentResponse
	Stats     DefenseStats
}

// CommentResponse represents a response to a reviewer comment
type CommentResponse struct {
	OriginalComment *github.PRComment
	Response        string
	Action          string // DEFEND or CONCEDE
}

// DefenseStats tracks defense statistics
type DefenseStats struct {
	CommentsAnalyzed int
	Defended         int
	Conceded         int
	Skipped          int
}

// CommentAnalysis is the AI analysis of a reviewer comment
type CommentAnalysis struct {
	IsValidIssue       bool     `json:"is_valid_issue"`
	ConfidenceValid    int      `json:"confidence_its_valid"`
	DefensePoints      []string `json:"defense_points"`
	WhatTheyMissed     string   `json:"what_they_missed"`
	RecommendedAction  string   `json:"recommended_action"`
}

// Defender handles PR comment defense
type Defender struct {
	config       *config.Config
	githubClient *github.Client
	aiClient     *ai.Client
}

// NewDefender creates a new defender instance
func NewDefender(cfg *config.Config) *Defender {
	return &Defender{
		config:       cfg,
		githubClient: github.NewClient(cfg.GitHubToken),
		aiClient:     ai.NewClient(cfg.AIApiURL, cfg.AIApiKey, cfg.AIModel),
	}
}

// Defend analyzes and responds to comments on your PR
func (d *Defender) Defend(prRef string, dryRun bool) (*DefenseResult, error) {
	ref, err := github.ParsePRReference(prRef)
	if err != nil {
		return nil, err
	}

	fmt.Printf("🛡️  Fetching PR #%d from %s/%s...\n", ref.Number, ref.Owner, ref.Repo)

	// Get PR details
	pr, err := d.githubClient.GetPR(ref)
	if err != nil {
		return nil, err
	}

	myUsername := d.getMyUsername()
	if pr.GetUser().GetLogin() != myUsername {
		fmt.Printf("⚠️  Warning: This PR was created by @%s, not you (@%s)\n", pr.GetUser().GetLogin(), myUsername)
	}

	fmt.Printf("📝 PR: %s\n", pr.GetTitle())

	// Get all comments
	comments, err := d.githubClient.GetPRComments(ref)
	if err != nil {
		return nil, err
	}

	// Filter to comments from others (not our own replies)
	var otherComments []*github.PRComment
	for _, c := range comments {
		if c.User != myUsername && c.InReplyTo == 0 {
			otherComments = append(otherComments, c)
		}
	}

	fmt.Printf("💬 Found %d comments from reviewers\n", len(otherComments))

	if len(otherComments) == 0 {
		fmt.Println("🎉 No comments to respond to!")
		return &DefenseResult{}, nil
	}

	result := &DefenseResult{
		Stats: DefenseStats{
			CommentsAnalyzed: len(otherComments),
		},
	}

	// Get file contents for context
	files, _ := d.githubClient.GetPRFiles(ref)
	fileContents := make(map[string]string)
	for _, f := range files {
		content, err := d.githubClient.GetFileContent(ref.Owner, ref.Repo, f.Filename, pr.GetHead().GetSHA())
		if err == nil {
			fileContents[f.Filename] = content
		}
	}

	// Analyze and respond to each comment
	for i, comment := range otherComments {
		fmt.Printf("\n📍 [%d/%d] Comment from @%s on %s\n", i+1, len(otherComments), comment.User, comment.Path)
		fmt.Printf("   \"%s\"\n", truncate(comment.Body, 80))

		// Get code context
		codeContext := ""
		if content, ok := fileContents[comment.Path]; ok {
			codeContext = extractContext(content, comment.Line)
		}

		// Analyze the comment
		analysis, err := d.analyzeComment(comment, codeContext)
		if err != nil {
			fmt.Printf("   ⚠️  Analysis failed: %v\n", err)
			result.Stats.Skipped++
			continue
		}

		// Generate response
		var response string
		if analysis.RecommendedAction == "CONCEDE" || analysis.ConfidenceValid >= 95 {
			fmt.Printf("   😤 Grudgingly conceding (they're %d%% right)\n", analysis.ConfidenceValid)
			response, err = d.generateConcession(comment.Body)
			result.Stats.Conceded++
		} else {
			fmt.Printf("   💪 Defending! (only %d%% valid, found %d defense points)\n",
				analysis.ConfidenceValid, len(analysis.DefensePoints))
			response, err = d.generateDefense(comment.Body, analysis)
			result.Stats.Defended++
		}

		if err != nil {
			fmt.Printf("   ⚠️  Response generation failed: %v\n", err)
			result.Stats.Skipped++
			continue
		}

		result.Responses = append(result.Responses, CommentResponse{
			OriginalComment: comment,
			Response:        response,
			Action:          analysis.RecommendedAction,
		})
	}

	// Post responses or show dry run
	if dryRun {
		fmt.Println("\n📋 DRY RUN - Would post the following responses:")
		fmt.Println("─────────────────────────────────────────")
		for _, r := range result.Responses {
			fmt.Printf("\n📍 In reply to @%s:\n", r.OriginalComment.User)
			fmt.Printf("   Original: \"%s\"\n", truncate(r.OriginalComment.Body, 60))
			fmt.Printf("   Action: %s\n", r.Action)
			fmt.Printf("   Response:\n%s\n", indent(r.Response, "   "))
		}
		fmt.Println("─────────────────────────────────────────")
	} else {
		fmt.Println("\n📤 Posting responses...")
		for i, r := range result.Responses {
			err := d.githubClient.ReplyToComment(ref, r.OriginalComment.ID, r.Response)
			if err != nil {
				fmt.Printf("   ⚠️  Failed to post response %d: %v\n", i+1, err)
			} else {
				fmt.Printf("   ✅ Posted response %d/%d\n", i+1, len(result.Responses))
			}
		}
	}

	// Print summary
	fmt.Printf("\n📊 Summary: %d defended, %d conceded, %d skipped\n",
		result.Stats.Defended, result.Stats.Conceded, result.Stats.Skipped)

	return result, nil
}

func (d *Defender) analyzeComment(comment *github.PRComment, codeContext string) (*CommentAnalysis, error) {
	prompt := GetCommentAnalysisPrompt(comment.Body, codeContext)

	messages := []ai.Message{
		ai.SystemMessage(GetDefenseSystemPrompt(d.config.WritingStyle)),
		ai.UserMessage(prompt),
	}

	response, err := d.aiClient.Chat(messages)
	if err != nil {
		return nil, err
	}

	// Extract JSON
	response = extractJSON(response)

	var analysis CommentAnalysis
	if err := json.Unmarshal([]byte(response), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse analysis: %w", err)
	}

	return &analysis, nil
}

func (d *Defender) generateDefense(comment string, analysis *CommentAnalysis) (string, error) {
	analysisJSON, _ := json.Marshal(analysis)

	prompt := GetDefenseResponsePrompt(comment, string(analysisJSON), d.config.WritingStyle)

	messages := []ai.Message{
		ai.SystemMessage(GetDefenseSystemPrompt(d.config.WritingStyle)),
		ai.UserMessage(prompt),
	}

	return d.aiClient.Chat(messages)
}

func (d *Defender) generateConcession(comment string) (string, error) {
	prompt := GetConcessionPrompt(comment, d.config.WritingStyle)

	messages := []ai.Message{
		ai.SystemMessage(GetDefenseSystemPrompt(d.config.WritingStyle)),
		ai.UserMessage(prompt),
	}

	return d.aiClient.Chat(messages)
}

func (d *Defender) getMyUsername() string {
	// In a real implementation, we'd fetch this from the GitHub API
	// For now, we'll use a placeholder that assumes you own the PR
	return "me"
}

// Helper functions

func extractJSON(response string) string {
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start != -1 && end != -1 && end > start {
		return response[start : end+1]
	}
	return response
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func indent(s string, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func extractContext(content string, line int) string {
	lines := strings.Split(content, "\n")
	start := line - 5
	end := line + 5

	if start < 0 {
		start = 0
	}
	if end > len(lines) {
		end = len(lines)
	}

	if line > 0 && line <= len(lines) {
		return strings.Join(lines[start:end], "\n")
	}
	return ""
}
