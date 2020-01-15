package git

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

type GitRepo struct {
	url      string
	repo     *git.Repository
	worktree *git.Worktree
	mu       sync.Mutex

	workDir     string
	user        string
	token       string
	commitName  string
	commitEmail string

	branch string
}

type Config struct {
	URL     string
	WorkDir string

	Username string
	Token    string

	AuthorName  string
	AuthorEmail string
}

func NewGitRepo(cfg Config) (*GitRepo, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, err
	}
	workDir := filepath.Join(cfg.WorkDir, u.Path)

	return &GitRepo{
		workDir:     workDir,
		url:         cfg.URL,
		user:        cfg.Username,
		token:       cfg.Token,
		commitName:  cfg.AuthorName,
		commitEmail: cfg.AuthorEmail,
		branch:      "master",
	}, nil
}

func (r *GitRepo) auth() *http.BasicAuth {
	return &http.BasicAuth{
		Username: r.user,
		Password: r.token,
	}
}

func (r *GitRepo) author() *object.Signature {
	return &object.Signature{
		Name:  r.commitName,
		Email: r.commitEmail,
		When:  time.Now(),
	}
}

func (r *GitRepo) Clone(ctx context.Context) error {
	repo, err := git.PlainCloneContext(ctx, r.workDir, false, &git.CloneOptions{
		URL:  r.url,
		Auth: r.auth(),
	})
	if err != nil {
		return err
	}

	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	r.repo = repo
	r.worktree = w

	return nil
}

func (r *GitRepo) Objects() (map[string]string, error) {
	m := make(map[string]string)
	head, err := r.repo.Head()
	if err != nil {
		return m, err
	}
	commit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		return m, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return m, err
	}
	for _, entry := range tree.Entries {
		content, _ := ioutil.ReadFile(filepath.Join(r.workDir, entry.Name))
		m[entry.Name] = string(content)
	}
	return m, nil
}

func (r *GitRepo) Open(ctx context.Context) error {
	repo, err := git.PlainOpen(r.workDir)
	if err != nil {
		return err
	}

	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	r.repo = repo
	r.worktree = w

	return nil
}

func (r *GitRepo) CloneOrOpen(ctx context.Context) error {
	_, err := os.Stat(r.workDir)
	switch {
	case os.IsNotExist(err):
		return r.Clone(ctx)
	default:
		err := r.Open(ctx)
		if err != nil {
			return err
		}
		return r.Pull(ctx)
	}
}

func (r *GitRepo) Pull(ctx context.Context) error {
	if err := r.worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(r.branch),
		Force:  true,
	}); err != nil {
		return err
	}

	err := r.worktree.PullContext(ctx, &git.PullOptions{
		Auth:          r.auth(),
		ReferenceName: plumbing.NewBranchReferenceName(r.branch),
		Force:         true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to pull: %v", err)
	}

	return nil
}

func (r *GitRepo) Path() string {
	return r.workDir
}

func (r *GitRepo) Push(ctx context.Context) error {
	if err := r.repo.PushContext(ctx, &git.PushOptions{
		Auth:       r.auth(),
		RemoteName: "origin",
	}); err != nil {
		return fmt.Errorf("failed to push %v: %v", r.branch, err)
	}

	return nil
}

func (r *GitRepo) Add(path string) error {
	_, err := r.worktree.Add(path)
	return err
}

func (r *GitRepo) Commit(msg string) error {
	_, err := r.worktree.Commit(msg, &git.CommitOptions{
		Author: r.author(),
	})
	return err
}

func (r *GitRepo) IsClean() bool {
	status, err := r.worktree.Status()
	if err != nil {
		return false
	}
	return status.IsClean()
}
