package gitlab

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/xanzy/go-gitlab"
	"golang.org/x/sync/errgroup"

	"github.com/mistwind/reviewdog"
	"github.com/mistwind/reviewdog/service/commentutil"
	"github.com/mistwind/reviewdog/service/serviceutil"
)

var _ reviewdog.CommentService = &PushCommitsCommenter{}

// PushCommitsCommenter is a comment service for GitLab commit in push.
//
// API:
//  https://docs.gitlab.com/ce/api/commits.html#post-comment-to-commit
//  POST /projects/:id/repository/commits/:sha/comments
type PushCommitsCommenter struct {
	cli      *gitlab.Client
	sha      string
	projects string

	sync.Mutex
	postComments []*reviewdog.Comment

	postedcs commentutil.PostedComments

	// wd is working directory relative to root of repository.
	wd string
}

// NewGitLabPushCommitsCommenter returns a new PushCommitsCommenter service.
// PushCommitsCommenter service needs git command in $PATH.
func NewGitLabPushCommitsCommenter(cli *gitlab.Client, owner, repo string, sha string) (*PushCommitsCommenter, error) {
	workDir, err := serviceutil.GitRelWorkdir()
	if err != nil {
		return nil, fmt.Errorf("PushCommitsCommenter needs 'git' command: %w", err)
	}
	return &PushCommitsCommenter{
		cli:      cli,
		sha:      sha,
		projects: owner + "/" + repo,
		wd:       workDir,
	}, nil
}

// Post accepts a comment and holds it. Flush method actually posts comments to
// GitLab in parallel.
func (g *PushCommitsCommenter) Post(_ context.Context, c *reviewdog.Comment) error {
	c.Result.Diagnostic.GetLocation().Path = filepath.ToSlash(
		filepath.Join(g.wd, c.Result.Diagnostic.GetLocation().GetPath()))
	g.Lock()
	defer g.Unlock()
	g.postComments = append(g.postComments, c)
	return nil
}

// Flush posts comments which has not been posted yet.
func (g *PushCommitsCommenter) Flush(ctx context.Context) error {
	g.Lock()
	defer g.Unlock()

	return g.postCommentsForEach(ctx)
}

func (g *PushCommitsCommenter) postCommentsForEach(ctx context.Context) error {
	var eg errgroup.Group
	for _, c := range g.postComments {
		c := c
		loc := c.Result.Diagnostic.GetLocation()
		lnum := int(loc.GetRange().GetStart().GetLine())
		body := commentutil.MarkdownComment(c)
		if !c.Result.InDiffFile || lnum == 0 || g.postedcs.IsPosted(c, lnum, body) {
			continue
		}
		eg.Go(func() error {
			prcomment := &gitlab.PostCommitCommentOptions{
				Note:     gitlab.String(body),
				Path:     gitlab.String(loc.GetPath()),
				Line:     gitlab.Int(lnum),
				LineType: gitlab.String("new"),
			}
			_, _, err := g.cli.Commits.PostCommitComment(g.projects, g.sha, prcomment, gitlab.WithContext(ctx))
			return err
		})
	}
	return eg.Wait()
}
