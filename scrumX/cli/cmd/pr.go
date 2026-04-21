package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/atharshah1/scrum/scrumX/cli/internal/api"
	"github.com/atharshah1/scrum/scrumX/cli/internal/utils"
	"github.com/spf13/cobra"
)

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Deterministic pull request and git-context workflows",
}

var prCreateIssueCmd = &cobra.Command{
	Use:   "create-issue",
	Short: "Create an issue from the current git context",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		cfg, err := cfgStore.Load()
		if err != nil {
			return err
		}
		projectID := strings.TrimSpace(mustString(cmd, "project-id"))
		if projectID == "" {
			projectID = strings.TrimSpace(cfg.CurrentProjectID)
		}
		if projectID == "" {
			return fmt.Errorf("--project-id is required (or set context current_project_id)")
		}
		ctx, err := detectGitContext(strings.TrimSpace(mustString(cmd, "base-ref")))
		if err != nil {
			return err
		}
		title := strings.TrimSpace(mustString(cmd, "title"))
		if title == "" {
			title = ctx.Title()
		}
		input := api.CreateIssueInput{
			Title:       title,
			ProjectID:   projectID,
			Description: ctx.Description(),
			Priority:    "medium",
			IssueType:   "task",
			AssigneeID:  nonEmpty(mustString(cmd, "assignee-id"), cfg.UserID),
			Labels:      ctx.Labels(),
		}
		if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
			fmt.Printf("title: %s\nproject: %s\nassignee: %s\nlabels: %s\n\n%s\n",
				input.Title,
				input.ProjectID,
				nonEmpty(input.AssigneeID, "<unset>"),
				strings.Join(input.Labels, ", "),
				input.Description,
			)
			return nil
		}
		issue, err := client.CreateIssueSmart(input)
		if err != nil {
			return err
		}
		fmt.Println(utils.SuccessText(fmt.Sprintf("Created issue %s from git context", issue.ID)))
		return nil
	},
}

var prLinkCmd = &cobra.Command{
	Use:   "link <issue-id>",
	Short: "Attach current git context to an existing issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		ctx, err := detectGitContext(strings.TrimSpace(mustString(cmd, "base-ref")))
		if err != nil {
			return err
		}
		comment := strings.Join([]string{
			fmt.Sprintf("Linked git context from %s", ctx.Repo),
			fmt.Sprintf("Branch: %s", ctx.Branch),
			fmt.Sprintf("Changed files: %s", strings.Join(ctx.ChangedFiles, ", ")),
			"",
			ctx.DiffStat,
		}, "\n")
		if _, err := client.AddComment(args[0], comment); err != nil {
			return err
		}
		fmt.Println(utils.SuccessText(fmt.Sprintf("Linked issue %s to %s", args[0], ctx.Branch)))
		return nil
	},
}

type gitContext struct {
	Repo         string
	Branch       string
	BaseRef      string
	ChangedFiles []string
	DiffStat     string
}

func (g gitContext) Title() string {
	branch := g.Branch
	if idx := strings.LastIndex(branch, "/"); idx >= 0 {
		branch = branch[idx+1:]
	}
	branch = strings.NewReplacer("-", " ", "_", " ", ".", " ").Replace(branch)
	branch = strings.Join(strings.Fields(branch), " ")
	if branch == "" {
		return "Review repository changes"
	}
	return strings.ToUpper(branch[:1]) + branch[1:]
}

func (g gitContext) Description() string {
	lines := []string{
		fmt.Sprintf("Repository: %s", g.Repo),
		fmt.Sprintf("Branch: %s", g.Branch),
	}
	if g.BaseRef != "" {
		lines = append(lines, fmt.Sprintf("Compared to: %s", g.BaseRef))
	}
	if len(g.ChangedFiles) > 0 {
		lines = append(lines, fmt.Sprintf("Changed files: %s", strings.Join(g.ChangedFiles, ", ")))
	}
	if strings.TrimSpace(g.DiffStat) != "" {
		lines = append(lines, "", "Diff summary:", g.DiffStat)
	}
	return strings.Join(lines, "\n")
}

func (g gitContext) Labels() []string {
	labels := []string{"repo:" + sanitizePRLabel(g.Repo)}
	seen := map[string]struct{}{labels[0]: {}}
	for _, file := range g.ChangedFiles {
		top := strings.TrimSpace(strings.Split(strings.TrimLeft(file, "/"), "/")[0])
		if top == "" || top == "." {
			continue
		}
		label := "area:" + sanitizePRLabel(top)
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		labels = append(labels, label)
		if len(labels) >= 4 {
			break
		}
	}
	sort.Strings(labels)
	return labels
}

func detectGitContext(baseRef string) (gitContext, error) {
	root, err := gitOutput("rev-parse", "--show-toplevel")
	if err != nil {
		return gitContext{}, fmt.Errorf("git context unavailable: %w", err)
	}
	branch, err := gitOutput("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return gitContext{}, err
	}
	base := nonEmpty(baseRef)
	if base == "" {
		if _, err := gitOutput("rev-parse", "--verify", "origin/main"); err == nil {
			base = "origin/main"
		}
	}
	diffArgs := []string{"diff", "--name-only"}
	statArgs := []string{"diff", "--stat"}
	if base != "" {
		diffArgs = append(diffArgs, base+"...HEAD")
		statArgs = append(statArgs, base+"...HEAD")
	}
	filesOutput, err := gitOutput(diffArgs...)
	if err != nil || strings.TrimSpace(filesOutput) == "" {
		filesOutput, _ = gitOutput("diff", "--name-only")
	}
	statOutput, _ := gitOutput(statArgs...)
	if strings.TrimSpace(statOutput) == "" {
		statOutput, _ = gitOutput("diff", "--stat")
	}
	return gitContext{
		Repo:         filepath.Base(strings.TrimSpace(root)),
		Branch:       strings.TrimSpace(branch),
		BaseRef:      base,
		ChangedFiles: parseCSV(strings.ReplaceAll(strings.TrimSpace(filesOutput), "\n", ",")),
		DiffStat:     strings.TrimSpace(statOutput),
	}, nil
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func sanitizePRLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("/", "-", "\\", "-", "_", "-", " ", "-", ".", "-")
	value = replacer.Replace(value)
	return strings.Trim(value, "-")
}

func init() {
	rootCmd.AddCommand(prCmd)
	prCmd.AddCommand(prCreateIssueCmd, prLinkCmd)
	prCreateIssueCmd.Flags().String("project-id", "", "Project UUID (defaults to active context)")
	prCreateIssueCmd.Flags().String("assignee-id", "", "Assignee user UUID (defaults to current user)")
	prCreateIssueCmd.Flags().String("title", "", "Override derived issue title")
	prCreateIssueCmd.Flags().String("base-ref", "", "Base git ref used to derive diff context")
	prCreateIssueCmd.Flags().Bool("dry-run", false, "Print the derived issue payload without creating it")
	prLinkCmd.Flags().String("base-ref", "", "Base git ref used to derive diff context")
}
