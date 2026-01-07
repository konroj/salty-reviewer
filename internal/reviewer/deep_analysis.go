package reviewer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/user/salty-reviewer/internal/ai"
	"github.com/user/salty-reviewer/internal/github"
)

// Issue represents a potential issue found in the first pass
type Issue struct {
	File              string `json:"file"`
	Line              int    `json:"line"`
	Code              string `json:"code"`
	Issue             string `json:"issue"`
	Confidence        int    `json:"confidence"`
	MightBeIntentional string `json:"might_be_intentional"`
}

// FirstPassResult is the result of initial issue scanning
type FirstPassResult struct {
	Issues []Issue `json:"issues"`
}

// DeepAnalysisResult is the result of analyzing a specific issue
type DeepAnalysisResult struct {
	StillAnIssue        bool   `json:"still_an_issue"`
	Confidence          int    `json:"confidence"`
	Reasoning           string `json:"reasoning"`
	PossibleAuthorIntent string `json:"possible_author_intent"`
	FinalVerdict        string `json:"final_verdict"`
}

// AnalyzedIssue combines the original issue with deep analysis
type AnalyzedIssue struct {
	Original Issue
	Analysis DeepAnalysisResult
}

// NitpickResult holds extra nitpicks for disliked reviewers
type NitpickResult struct {
	Nitpicks []struct {
		File    string `json:"file"`
		Line    int    `json:"line"`
		Comment string `json:"comment"`
	} `json:"nitpicks"`
}

// Analyzer handles deep code analysis
type Analyzer struct {
	aiClient     *ai.Client
	githubClient *github.Client
}

// NewAnalyzer creates a new deep analyzer
func NewAnalyzer(aiClient *ai.Client, githubClient *github.Client) *Analyzer {
	return &Analyzer{
		aiClient:     aiClient,
		githubClient: githubClient,
	}
}

// FirstPass identifies potential issues in the diff
func (a *Analyzer) FirstPass(files []*github.FileChange) (*FirstPassResult, error) {
	// Combine all diffs into one for the first pass
	var diffBuilder strings.Builder
	for _, f := range files {
		diffBuilder.WriteString(fmt.Sprintf("\n--- %s ---\n", f.Filename))
		diffBuilder.WriteString(f.Patch)
		diffBuilder.WriteString("\n")
	}

	messages := []ai.Message{
		ai.SystemMessage(GetFirstPassPrompt()),
		ai.UserMessage(diffBuilder.String()),
	}

	response, err := a.aiClient.Chat(messages)
	if err != nil {
		return nil, fmt.Errorf("AI first pass failed: %w", err)
	}

	// Parse JSON response
	response = extractJSON(response)
	var result FirstPassResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse first pass result: %w (response: %s)", err, response)
	}

	return &result, nil
}

// DeepAnalyze performs deep analysis on a specific issue
func (a *Analyzer) DeepAnalyze(issue Issue, ref *github.PRReference, pr *github.PullRequest) (*DeepAnalysisResult, error) {
	// Get full file content
	fullContent, err := a.githubClient.GetFileContent(ref.Owner, ref.Repo, issue.File, pr.GetHead().GetSHA())
	if err != nil {
		// If we can't get the file, still try with available info
		fullContent = "(File content unavailable)"
	}

	// Get related files
	related, _ := a.githubClient.GetRelatedFiles(ref.Owner, ref.Repo, issue.File, pr.GetHead().GetSHA())
	var relatedContent strings.Builder
	for _, r := range related {
		content, err := a.githubClient.GetFileContent(ref.Owner, ref.Repo, r, pr.GetHead().GetSHA())
		if err == nil {
			relatedContent.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", r, content))
		}
	}

	issueDesc := fmt.Sprintf("File: %s, Line: %d\nCode: %s\nIssue: %s",
		issue.File, issue.Line, issue.Code, issue.Issue)

	prompt := GetDeepAnalysisPrompt(issueDesc, fullContent, relatedContent.String())

	messages := []ai.Message{
		ai.SystemMessage("You are a thoughtful code reviewer who considers context before judging."),
		ai.UserMessage(prompt),
	}

	response, err := a.aiClient.Chat(messages)
	if err != nil {
		return nil, fmt.Errorf("AI deep analysis failed: %w", err)
	}

	response = extractJSON(response)
	var result DeepAnalysisResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse deep analysis: %w", err)
	}

	return &result, nil
}

// GenerateExtraNitpicks creates additional nitpicky comments
func (a *Analyzer) GenerateExtraNitpicks(files []*github.FileChange, existingComments []string) (*NitpickResult, error) {
	var diffBuilder strings.Builder
	for _, f := range files {
		diffBuilder.WriteString(fmt.Sprintf("\n--- %s ---\n", f.Filename))
		diffBuilder.WriteString(f.Patch)
	}

	prompt := GetExtraNitpickPrompt(diffBuilder.String(), strings.Join(existingComments, "\n"))

	messages := []ai.Message{
		ai.SystemMessage("You are an extremely pedantic code reviewer who finds issues with everything."),
		ai.UserMessage(prompt),
	}

	response, err := a.aiClient.Chat(messages)
	if err != nil {
		return nil, fmt.Errorf("AI nitpick generation failed: %w", err)
	}

	response = extractJSON(response)
	var result NitpickResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse nitpicks: %w", err)
	}

	return &result, nil
}

// extractJSON tries to extract JSON from a response that might have extra text
func extractJSON(response string) string {
	// Find the first { and last }
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")

	if start != -1 && end != -1 && end > start {
		return response[start : end+1]
	}

	return response
}
