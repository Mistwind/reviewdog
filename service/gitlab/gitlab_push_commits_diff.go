package gitlab

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/xanzy/go-gitlab"

	"github.com/reviewdog/reviewdog"
	"github.com/reviewdog/reviewdog/service/serviceutil"
)

var _ reviewdog.DiffService = &PushCommitsDiff{}

// PushCommitsDiff is a diff service for GitLab Push Commits.
type PushCommitsDiff struct {
	cli       *gitlab.Client
	sha       string
	beforeSHA string
	projects  string

	// wd is working directory relative to root of repository.
	wd string
}

// NewGitLabPushCommitsDiff returns a new PushCommitsDiff service.
// itLabPushCommitsDiff service needs git command in $PATH.
func NewGitLabPushCommitsDiff(cli *gitlab.Client, owner, repo string, sha string, beforeSHA string) (*PushCommitsDiff, error) {
	workDir, err := serviceutil.GitRelWorkdir()
	if err != nil {
		return nil, fmt.Errorf("PushCommitsDiff needs 'git' command: %w", err)
	}
	return &PushCommitsDiff{
		cli:       cli,
		sha:       sha,
		beforeSHA: beforeSHA,
		projects:  owner + "/" + repo,
		wd:        workDir,
	}, nil
}

// Diff returns a diff of PushCommits. It runs `git diff` locally instead of
// diff_url of GitLab Merge Request because diff of diff_url is not suited for
// comment API in a sense that diff of diff_url is equivalent to
// `git diff --no-renames`, we want diff which is equivalent to
// `git diff --find-renames`.
// git diff old new
func (g *PushCommitsDiff) Diff(ctx context.Context) ([]byte, error) {
	return g.gitDiff(ctx, g.beforeSHA, g.sha)
}

func (g *PushCommitsDiff) gitDiff(_ context.Context, baseSha, targetSha string) ([]byte, error) {
	bytes, err := exec.Command("git", "diff", "--find-renames", baseSha, targetSha).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run git diff: %w", err)
	}
	return bytes, nil
}

// Strip returns 1 as a strip of git diff.
func (g *PushCommitsDiff) Strip() int {
	return 1
}
