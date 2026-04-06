package node

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func ensureRepo(ctx context.Context, repoURL, repoDir string) error {
	if repoURL == "" {
		return fmt.Errorf("missing repo URL")
	}
	if repoDir == "" {
		return fmt.Errorf("missing repo directory")
	}

	if exists, err := pathExists(repoDir); err != nil {
		return err
	} else if !exists {
		return runGit(ctx, "", "clone", repoURL, repoDir)
	}

	if ok, err := isGitRepo(repoDir); err != nil {
		return err
	} else if ok {
		return nil
	}

	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("%s exists and is not an empty git repository", repoDir)
	}

	return runGit(ctx, "", "clone", repoURL, repoDir)
}

func syncRepo(ctx context.Context, repoDir string) error {
	branch, err := currentBranch(ctx, repoDir)
	if err != nil || branch == "" || branch == "HEAD" {
		branch = "main"
	}

	if err := runGit(ctx, repoDir, "fetch", "--prune", "--all"); err != nil {
		return err
	}
	if err := resetToRemoteBranch(ctx, repoDir, branch); err != nil {
		return err
	}
	return runGit(ctx, repoDir, "clean", "-fd")
}

func hardResetRepo(ctx context.Context, repoDir string) error {
	branch, err := currentBranch(ctx, repoDir)
	if err != nil || branch == "" || branch == "HEAD" {
		branch = "main"
	}
	if err := resetToRemoteBranch(ctx, repoDir, branch); err != nil {
		return err
	}
	return runGit(ctx, repoDir, "clean", "-fd")
}

func resetToRemoteBranch(ctx context.Context, repoDir, branch string) error {
	candidates := []string{branch}
	if branch != "main" {
		candidates = append(candidates, "main")
	}
	if branch != "master" {
		candidates = append(candidates, "master")
	}
	for _, candidate := range candidates {
		if err := runGit(ctx, repoDir, "reset", "--hard", "origin/"+candidate); err == nil {
			return nil
		}
	}
	return fmt.Errorf("unable to reset to remote branch candidates %v", candidates)
}

func currentBranch(ctx context.Context, repoDir string) (string, error) {
	out, err := runGitOutput(ctx, repoDir, "branch", "--show-current")
	if err == nil && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out), nil
	}

	out, err = runGitOutput(ctx, repoDir, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		branch := strings.TrimSpace(strings.TrimPrefix(out, "origin/"))
		if branch != "" {
			return branch, nil
		}
	}
	return "main", nil
}

func isGitRepo(repoDir string) (bool, error) {
	cmd := exec.Command("git", "-C", repoDir, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

func runGit(ctx context.Context, repoDir string, args ...string) error {
	_, err := runGitOutput(ctx, repoDir, args...)
	return err
}

func runGitOutput(ctx context.Context, repoDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if repoDir != "" {
		cmd.Dir = repoDir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
