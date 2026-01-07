# Salty Code Reviewer

> *"I'm sure you already know this, but your code could be better."*

Fear of code review comments keeping you up at night? **Good.** That fear will make you write better PRs. More precise. More thoughtful. Fully engaged. Because you know Salty is watching. And Salty *will* comment.

A **satirical** GitHub PR review assistant that brings personality (and perhaps a bit too much of it) to code reviews.

**DISCLAIMER**: This is a satirical tool for entertainment purposes. Using it on real PRs may result in HR meetings, lost friendships, and/or becoming *that person* on your team. You have been warned.

## Features

### Deep Code Review

Most AI reviewers skim your code like a recruiter skims resumes. Salty actually *reads* it:

1. **First Pass**: Scans for things that look wrong (like your senior dev before coffee)
2. **Deep Analysis**: Before mass commenting, asks itself:
   - "Wait, why would someone do this?"
   - "Is there some 3am-deadline context I'm missing?"
   - "Could this actually be... intentional?"
3. **Confidence Scoring**: Only opens its mouth if 80%+ sure. Unlike *some* reviewers.
4. **Context Awareness**: Actually reads the whole file. Revolutionary, we know.

### Configurable Personality

#### Writing Styles

- **Corporate**: *"Per our established best practices, this implementation may benefit from stakeholder alignment..."*
- **Passive Aggressive**: *"I'm sure you already know this, but just in case..."*
- **Tech Bro**: *"Actually, if you look at the Big O complexity here..."*
- **Academic**: *"According to Martin Fowler (2018), this violates the principle established in..."*

#### Nitpicky Levels (1-10)

- **Level 1**: Only critical bugs and security issues
- **Level 5**: Standard code review
- **Level 10**: *Comments on whitespace, questions every variable name, demands documentation for every function*

### Reviewer Bias

Configure who you like and don't like:
- **Liked Reviewers**: Lower nitpicky threshold, benefit of the doubt
- **Disliked Reviewers**: +3 nitpicky boost, extra comments generated

### PR Defense Mode

The **defend** command helps you respond to comments on your PRs:

- Assumes every comment is wrong until proven otherwise
- Only concedes if the issue is 100% undeniable
- Generates lengthy rebuttals with:
  - Technical justifications
  - Edge cases the reviewer "didn't consider"
  - References to "industry standards"
  - Subtle implications they don't understand the full context

## Installation

```bash
# Clone the repository
git clone https://github.com/user/salty-reviewer.git
cd salty-reviewer

# Build
go build -o salty ./cmd/salty

# Install globally (optional)
go install ./cmd/salty
```

## Configuration

### Quick Setup

```bash
salty init
```

This will prompt you for:
- GitHub Personal Access Token
- AI API credentials (OpenAI-compatible)
- Writing style preference
- Nitpicky level

### Manual Configuration

Copy the example config:

```bash
mkdir -p ~/.salty-reviewer
cp config.example.yaml ~/.salty-reviewer/config.yaml
```

Edit the config file with your settings.

### AI API Options

Salty works with any OpenAI-compatible API:

```yaml
# OpenAI
ai_api_url: https://api.openai.com/v1
ai_model: gpt-4

# Azure OpenAI
ai_api_url: https://your-resource.openai.azure.com/openai/deployments/your-deployment
ai_model: gpt-4

# Local models (Ollama, LM Studio, etc.)
ai_api_url: http://localhost:11434/v1
ai_model: llama2
```

## Usage

### Review a PR

```bash
# Basic review
salty review owner/repo#123

# Using full URL
salty review https://github.com/owner/repo/pull/123

# Dry run (see what would be posted)
salty review --dry-run owner/repo#123
```

### Defend Your PR

```bash
# Respond to all reviewer comments
salty defend owner/repo#123

# Dry run (see responses without posting)
salty defend --dry-run owner/repo#123
```

### Manage Configuration

```bash
# View current settings
salty config show

# Set writing style
salty config set writing_style tech_bro

# Crank up the nitpicking
salty config set nitpicky_level 9

# Add someone to your "special" list
salty config add disliked_reviewer that_guy
salty config add liked_reviewer cool_dev
```

## Example Output

### Review Mode

```
$ salty review myorg/myrepo#42

Fetching PR #42 from myorg/myrepo...
PR by @junior_dev: Add user authentication

Author is disliked - extra scrutiny (nitpicky: 8)
Reviewing 5 changed files...

First pass: identifying potential issues...
   Found 12 potential issues

Deep analysis: verifying each issue...
   [1/12] Analyzing: auth.go (line 42)...
      Confirmed (confidence: 92%)
   [2/12] Analyzing: auth.go (line 67)...
      Skipped (confidence: 45%, threshold: 50%)
   ...

Formatting comments...
Generating extra nitpicks for disliked reviewer...
   Added 3 extra nitpicks

Review posted with 8 comments
```

### Defend Mode

```
$ salty defend myorg/myrepo#99

Fetching PR #99 from myorg/myrepo...
PR: Implement caching layer

Found 3 comments from reviewers

[1/3] Comment from @senior_dev on cache.go
   "Why not use Redis here?"
   Defending! (only 35% valid, found 4 defense points)

[2/3] Comment from @that_guy on cache.go
   "This will cause a memory leak"
   Grudgingly conceding (they're 98% right)

Summary: 2 defended, 1 conceded, 0 skipped
```

## Project Structure

```
salty-reviewer/
├── cmd/salty/           # CLI entry point
├── internal/
│   ├── config/          # Configuration management
│   ├── github/          # GitHub API client
│   ├── ai/              # Generic AI client
│   ├── reviewer/        # Review logic & prompts
│   └── defender/        # PR defense logic & prompts
├── config.example.yaml
└── README.md
```

## Legal Disclaimer

This tool is **satire**. It's designed to be funny, not to actually be used in production code reviews (unless you really want to).

By using this tool, you acknowledge that:
- Your teammates may stop talking to you
- Your PRs may never get approved again
- You might accidentally become the villain in someone's "worst coworker" story

Use responsibly. Or don't. We're not your mom.

## License

MIT - because even satire deserves to be free.
