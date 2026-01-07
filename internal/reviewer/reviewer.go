package reviewer

import (
	"fmt"
	"strings"

	"github.com/user/salty-reviewer/internal/ai"
	"github.com/user/salty-reviewer/internal/config"
	"github.com/user/salty-reviewer/internal/github"
)

// ReviewResult is the final output of a review
type ReviewResult struct {
	Summary  string
	Comments []*github.ReviewComment
	Stats    ReviewStats
}

// ReviewStats tracks review statistics
type ReviewStats struct {
	FilesReviewed    int
	IssuesFound      int
	IssuesAfterDeep  int
	NitpicksAdded    int
	CommentsPosted   int
}

// Reviewer orchestrates the code review process
type Reviewer struct {
	config       *config.Config
	githubClient *github.Client
	aiClient     *ai.Client
	analyzer     *Analyzer
}

// NewReviewer creates a new reviewer instance
func NewReviewer(cfg *config.Config) *Reviewer {
	ghClient := github.NewClient(cfg.GitHubToken)
	aiClient := ai.NewClient(cfg.AIApiURL, cfg.AIApiKey, cfg.AIModel)
	analyzer := NewAnalyzer(aiClient, ghClient)

	return &Reviewer{
		config:       cfg,
		githubClient: ghClient,
		aiClient:     aiClient,
		analyzer:     analyzer,
	}
}

// Review performs a full code review on a PR
func (r *Reviewer) Review(prRef string, dryRun bool) (*ReviewResult, error) {
	ref, err := github.ParsePRReference(prRef)
	if err != nil {
		return nil, err
	}

	fmt.Printf("🔍 Fetching PR #%d from %s/%s...\n", ref.Number, ref.Owner, ref.Repo)

	// Get PR details
	pr, err := r.githubClient.GetPR(ref)
	if err != nil {
		return nil, err
	}

	author := pr.GetUser().GetLogin()
	fmt.Printf("📝 PR by @%s: %s\n", author, pr.GetTitle())

	// Calculate effective nitpicky level based on author
	effectiveNitpicky := r.config.NitpickyLevel + r.config.GetReviewerBias(author)
	if effectiveNitpicky < 1 {
		effectiveNitpicky = 1
	}
	if effectiveNitpicky > 10 {
		effectiveNitpicky = 10
	}

	if r.config.IsLikedReviewer(author) {
		fmt.Printf("💚 Author is liked - going easy (nitpicky: %d)\n", effectiveNitpicky)
	} else if r.config.IsDislikedReviewer(author) {
		fmt.Printf("🔴 Author is disliked - extra scrutiny (nitpicky: %d)\n", effectiveNitpicky)
	}

	// Get changed files
	files, err := r.githubClient.GetPRFiles(ref)
	if err != nil {
		return nil, err
	}

	fmt.Printf("📁 Reviewing %d changed files...\n", len(files))

	result := &ReviewResult{
		Stats: ReviewStats{
			FilesReviewed: len(files),
		},
	}

	// First pass: identify potential issues
	fmt.Println("🔎 First pass: identifying potential issues...")
	firstPass, err := r.analyzer.FirstPass(files)
	if err != nil {
		return nil, fmt.Errorf("first pass failed: %w", err)
	}

	result.Stats.IssuesFound = len(firstPass.Issues)
	fmt.Printf("   Found %d potential issues\n", len(firstPass.Issues))

	// Deep analysis for each issue
	fmt.Println("🔬 Deep analysis: verifying each issue...")
	var confirmedIssues []AnalyzedIssue

	for i, issue := range firstPass.Issues {
		fmt.Printf("   [%d/%d] Analyzing: %s (line %d)...\n", i+1, len(firstPass.Issues), issue.File, issue.Line)

		analysis, err := r.analyzer.DeepAnalyze(issue, ref, pr)
		if err != nil {
			fmt.Printf("      ⚠️  Deep analysis failed: %v\n", err)
			continue
		}

		// Apply confidence threshold based on nitpicky level
		threshold := 90 - (effectiveNitpicky * 5) // Level 1 = 85%, Level 10 = 40%
		if analysis.Confidence >= threshold && analysis.FinalVerdict == "COMMENT" {
			confirmedIssues = append(confirmedIssues, AnalyzedIssue{
				Original: issue,
				Analysis: *analysis,
			})
			fmt.Printf("      ✓ Confirmed (confidence: %d%%)\n", analysis.Confidence)
		} else {
			fmt.Printf("      ✗ Skipped (confidence: %d%%, threshold: %d%%)\n", analysis.Confidence, threshold)
		}
	}

	result.Stats.IssuesAfterDeep = len(confirmedIssues)
	fmt.Printf("   %d issues confirmed after deep analysis\n", len(confirmedIssues))

	// Generate comments with proper styling
	fmt.Println("✍️  Formatting comments...")
	for _, ci := range confirmedIssues {
		comment, err := r.formatComment(ci)
		if err != nil {
			fmt.Printf("   ⚠️  Failed to format comment: %v\n", err)
			continue
		}

		result.Comments = append(result.Comments, &github.ReviewComment{
			Path: ci.Original.File,
			Line: ci.Original.Line,
			Body: comment,
			Side: "RIGHT",
		})
	}

	// Extra nitpicks for disliked reviewers
	if r.config.IsDislikedReviewer(author) {
		fmt.Println("😈 Generating extra nitpicks for disliked reviewer...")
		existingCommentBodies := make([]string, len(result.Comments))
		for i, c := range result.Comments {
			existingCommentBodies[i] = c.Body
		}

		nitpicks, err := r.analyzer.GenerateExtraNitpicks(files, existingCommentBodies)
		if err == nil && nitpicks != nil {
			for _, np := range nitpicks.Nitpicks {
				result.Comments = append(result.Comments, &github.ReviewComment{
					Path: np.File,
					Line: np.Line,
					Body: np.Comment,
					Side: "RIGHT",
				})
				result.Stats.NitpicksAdded++
			}
			fmt.Printf("   Added %d extra nitpicks\n", len(nitpicks.Nitpicks))
		}
	}

	// Generate summary
	result.Summary = r.generateSummary(result, pr)

	// Post the review (unless dry run)
	if dryRun {
		fmt.Println("\n📋 DRY RUN - Would post the following review:")
		fmt.Println("─────────────────────────────────────────")
		fmt.Println(result.Summary)
		for _, c := range result.Comments {
			fmt.Printf("\n📍 %s:%d\n%s\n", c.Path, c.Line, c.Body)
		}
		fmt.Println("─────────────────────────────────────────")
	} else {
		fmt.Println("📤 Posting review...")
		event := "COMMENT"
		if len(result.Comments) > 0 && effectiveNitpicky >= 7 {
			event = "REQUEST_CHANGES"
		}

		if err := r.githubClient.PostReview(ref, result.Summary, event, result.Comments); err != nil {
			return nil, fmt.Errorf("failed to post review: %w", err)
		}
		result.Stats.CommentsPosted = len(result.Comments)
		fmt.Printf("✅ Review posted with %d comments\n", len(result.Comments))
	}

	return result, nil
}

func (r *Reviewer) formatComment(issue AnalyzedIssue) (string, error) {
	issueDesc := fmt.Sprintf("Issue: %s\nCode: %s", issue.Original.Issue, issue.Original.Code)
	analysisDesc := fmt.Sprintf("Reasoning: %s", issue.Analysis.Reasoning)

	prompt := GetCommentFormattingPrompt(issueDesc, analysisDesc, r.config.WritingStyle)

	messages := []ai.Message{
		ai.SystemMessage(GetSystemPrompt(r.config.WritingStyle, r.config.NitpickyLevel)),
		ai.UserMessage(prompt),
	}

	return r.aiClient.Chat(messages)
}

func (r *Reviewer) generateSummary(result *ReviewResult, pr *github.PullRequest) string {
	var sb strings.Builder

	switch r.config.WritingStyle {
	case config.StyleCorporate:
		sb.WriteString("## Code Review Summary\n\n")
		sb.WriteString("Thank you for your contribution to this project. ")
		sb.WriteString("Please find below my observations regarding this pull request.\n\n")
	case config.StylePassiveAggressive:
		sb.WriteString("## Review Notes\n\n")
		sb.WriteString("I've had a chance to look over this PR. ")
		sb.WriteString("I'm sure most of my comments are probably unnecessary, but just in case...\n\n")
	case config.StyleTechBro:
		sb.WriteString("## Quick Review 🚀\n\n")
		sb.WriteString("Took a pass through this. Some thoughts below. ")
		sb.WriteString("Let's iterate quickly on these and ship it.\n\n")
	case config.StyleAcademic:
		sb.WriteString("## Review Commentary\n\n")
		sb.WriteString("Upon examination of the proposed changes, ")
		sb.WriteString("several observations warrant discussion.\n\n")
	}

	sb.WriteString(fmt.Sprintf("**Files reviewed:** %d\n", result.Stats.FilesReviewed))
	sb.WriteString(fmt.Sprintf("**Comments:** %d\n\n", len(result.Comments)))

	if len(result.Comments) == 0 {
		switch r.config.WritingStyle {
		case config.StyleCorporate:
			sb.WriteString("No significant issues identified at this time. Approved pending standard verification procedures.")
		case config.StylePassiveAggressive:
			sb.WriteString("I couldn't find anything to comment on. I'm sure it's fine. Probably.")
		case config.StyleTechBro:
			sb.WriteString("LGTM! Ship it. 🚀")
		case config.StyleAcademic:
			sb.WriteString("The implementation appears sound. No substantive concerns identified.")
		}
	}

	return sb.String()
}
