package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/salty-reviewer/internal/config"
	"github.com/user/salty-reviewer/internal/defender"
	"github.com/user/salty-reviewer/internal/reviewer"
)

var (
	dryRun      bool
	interactive bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "salty",
		Short: "🧂 Salty Code Reviewer - The satirical PR review assistant",
		Long: `Salty Code Reviewer is a satirical GitHub PR review assistant that:
- Reviews PRs with deep analysis and configurable personality
- Defends your PRs against "unreasonable" reviewer comments
- Supports multiple writing styles and nitpicky levels`,
	}

	// Init command
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize salty-reviewer configuration",
		RunE:  runInit,
	}

	// Review command
	reviewCmd := &cobra.Command{
		Use:   "review <pr-reference>",
		Short: "Review a pull request",
		Long: `Review a pull request with deep analysis.

Examples:
  salty review owner/repo#123
  salty review https://github.com/owner/repo/pull/123
  salty review --dry-run owner/repo#42`,
		Args: cobra.ExactArgs(1),
		RunE: runReview,
	}
	reviewCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be posted without actually posting")
	reviewCmd.Flags().BoolVar(&interactive, "interactive", false, "Confirm each comment before posting")

	// Defend command
	defendCmd := &cobra.Command{
		Use:   "defend <pr-reference>",
		Short: "Defend your PR against reviewer comments",
		Long: `Analyze and respond to comments on your PR.

The defender will:
- Assume each comment is wrong until proven otherwise
- Only concede if an issue is 100% undeniable
- Generate detailed rebuttals for everything else

Examples:
  salty defend owner/repo#123
  salty defend --dry-run https://github.com/owner/repo/pull/42`,
		Args: cobra.ExactArgs(1),
		RunE: runDefend,
	}
	defendCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be posted without actually posting")
	defendCmd.Flags().BoolVar(&interactive, "interactive", false, "Confirm each response before posting")

	// Config command
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}

	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE:  runConfigShow,
	}

	configSetCmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a configuration value.

Available keys:
  writing_style      - corporate, passive_aggressive, tech_bro, academic
  nitpicky_level     - 1-10 (1=lenient, 10=maximum nitpicking)
  github_token       - Your GitHub personal access token
  ai_api_url         - AI API endpoint (OpenAI-compatible)
  ai_api_key         - AI API key
  ai_model           - AI model name

Examples:
  salty config set writing_style tech_bro
  salty config set nitpicky_level 8`,
		Args: cobra.ExactArgs(2),
		RunE: runConfigSet,
	}

	configAddCmd := &cobra.Command{
		Use:   "add <list> <username>",
		Short: "Add a user to liked or disliked list",
		Long: `Add a user to the liked or disliked reviewers list.

Lists:
  liked_reviewer     - Go easy on these reviewers
  disliked_reviewer  - Extra scrutiny for these reviewers

Examples:
  salty config add liked_reviewer cool_dev
  salty config add disliked_reviewer that_guy`,
		Args: cobra.ExactArgs(2),
		RunE: runConfigAdd,
	}

	configCmd.AddCommand(configShowCmd, configSetCmd, configAddCmd)
	rootCmd.AddCommand(initCmd, reviewCmd, defendCmd, configCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Println("🧂 Salty Code Reviewer - Initial Setup")
	fmt.Println("─────────────────────────────────────────")

	reader := bufio.NewReader(os.Stdin)

	cfg := config.DefaultConfig()

	// GitHub token
	fmt.Print("\nGitHub Personal Access Token: ")
	token, _ := reader.ReadString('\n')
	cfg.GitHubToken = strings.TrimSpace(token)

	// AI API settings
	fmt.Print("\nAI API URL (default: https://api.openai.com/v1): ")
	apiURL, _ := reader.ReadString('\n')
	apiURL = strings.TrimSpace(apiURL)
	if apiURL != "" {
		cfg.AIApiURL = apiURL
	}

	fmt.Print("AI API Key: ")
	apiKey, _ := reader.ReadString('\n')
	cfg.AIApiKey = strings.TrimSpace(apiKey)

	fmt.Print("AI Model (default: gpt-4): ")
	model, _ := reader.ReadString('\n')
	model = strings.TrimSpace(model)
	if model != "" {
		cfg.AIModel = model
	}

	// Writing style
	fmt.Println("\nWriting Styles:")
	fmt.Println("  1. corporate         - \"Per our established best practices...\"")
	fmt.Println("  2. passive_aggressive - \"I'm sure you already know this, but...\"")
	fmt.Println("  3. tech_bro          - \"Actually, if you look at the Big O...\"")
	fmt.Println("  4. academic          - \"According to Martin Fowler (2018)...\"")
	fmt.Print("Choose style (1-4, default: 2): ")
	styleChoice, _ := reader.ReadString('\n')
	styleChoice = strings.TrimSpace(styleChoice)
	switch styleChoice {
	case "1":
		cfg.WritingStyle = config.StyleCorporate
	case "3":
		cfg.WritingStyle = config.StyleTechBro
	case "4":
		cfg.WritingStyle = config.StyleAcademic
	default:
		cfg.WritingStyle = config.StylePassiveAggressive
	}

	// Nitpicky level
	fmt.Print("\nNitpicky level (1-10, default: 5): ")
	levelStr, _ := reader.ReadString('\n')
	levelStr = strings.TrimSpace(levelStr)
	if levelStr != "" {
		if level, err := strconv.Atoi(levelStr); err == nil && level >= 1 && level <= 10 {
			cfg.NitpickyLevel = level
		}
	}

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	configPath, _ := config.ConfigPath()
	fmt.Printf("\n✅ Configuration saved to %s\n", configPath)
	fmt.Println("\nYou can now use:")
	fmt.Println("  salty review owner/repo#123    - Review a PR")
	fmt.Println("  salty defend owner/repo#123    - Defend your PR")
	fmt.Println("  salty config show              - View settings")

	return nil
}

func runReview(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	r := reviewer.NewReviewer(cfg)
	_, err = r.Review(args[0], dryRun)
	return err
}

func runDefend(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	d := defender.NewDefender(cfg)
	_, err = d.Defend(args[0], dryRun)
	return err
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		// Show defaults if no config exists
		fmt.Println("⚠️  No config found. Run 'salty init' to create one.")
		fmt.Println("\nDefault settings:")
		cfg = config.DefaultConfig()
	}

	fmt.Println("🧂 Salty Code Reviewer Configuration")
	fmt.Println("─────────────────────────────────────────")
	fmt.Printf("Writing Style:      %s\n", cfg.WritingStyle)
	fmt.Printf("Nitpicky Level:     %d/10\n", cfg.NitpickyLevel)
	fmt.Printf("AI API URL:         %s\n", cfg.AIApiURL)
	fmt.Printf("AI Model:           %s\n", cfg.AIModel)
	fmt.Printf("GitHub Token:       %s\n", maskToken(cfg.GitHubToken))
	fmt.Printf("AI API Key:         %s\n", maskToken(cfg.AIApiKey))
	fmt.Printf("Liked Reviewers:    %v\n", cfg.LikedReviewers)
	fmt.Printf("Disliked Reviewers: %v\n", cfg.DislikedReviewers)

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	key := args[0]
	value := args[1]

	switch key {
	case "writing_style":
		switch value {
		case "corporate":
			cfg.WritingStyle = config.StyleCorporate
		case "passive_aggressive":
			cfg.WritingStyle = config.StylePassiveAggressive
		case "tech_bro":
			cfg.WritingStyle = config.StyleTechBro
		case "academic":
			cfg.WritingStyle = config.StyleAcademic
		default:
			return fmt.Errorf("invalid writing style: %s", value)
		}
	case "nitpicky_level":
		level, err := strconv.Atoi(value)
		if err != nil || level < 1 || level > 10 {
			return fmt.Errorf("nitpicky_level must be 1-10")
		}
		cfg.NitpickyLevel = level
	case "github_token":
		cfg.GitHubToken = value
	case "ai_api_url":
		cfg.AIApiURL = value
	case "ai_api_key":
		cfg.AIApiKey = value
	case "ai_model":
		cfg.AIModel = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	fmt.Printf("✅ Set %s = %s\n", key, value)
	return nil
}

func runConfigAdd(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	list := args[0]
	username := args[1]

	switch list {
	case "liked_reviewer":
		cfg.AddLikedReviewer(username)
		fmt.Printf("✅ Added @%s to liked reviewers (will go easy on them)\n", username)
	case "disliked_reviewer":
		cfg.AddDislikedReviewer(username)
		fmt.Printf("✅ Added @%s to disliked reviewers (extra scrutiny mode)\n", username)
	default:
		return fmt.Errorf("unknown list: %s (use liked_reviewer or disliked_reviewer)", list)
	}

	return cfg.Save()
}

func maskToken(token string) string {
	if token == "" {
		return "(not set)"
	}
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "..." + token[len(token)-4:]
}
