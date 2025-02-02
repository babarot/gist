package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

type Repo struct {
	url      string
	repo     *git.Repository
	worktree *git.Worktree

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

func NewRepo(cfg Config) (*Repo, error) {
	// u, err := url.Parse(cfg.URL)
	// if err != nil {
	// 	return nil, err
	// }
	// workDir := filepath.Join(cfg.WorkDir, u.Path)

	if cfg.WorkDir == "" {
		return &Repo{}, errors.New("workdir is empty")
	}
	if cfg.URL == "" {
		return &Repo{}, errors.New("URL is empty")
	}
	if cfg.Token == "" {
		return &Repo{}, errors.New("token is empty")
	}
	if cfg.Username == "" {
		return &Repo{}, errors.New("user is empty")
	}

	return &Repo{
		workDir:     cfg.WorkDir,
		url:         cfg.URL,
		user:        cfg.Username,
		token:       cfg.Token,
		commitName:  cfg.AuthorName,
		commitEmail: cfg.AuthorEmail,
		branch:      "master",
	}, nil
}

func (r *Repo) auth() *http.BasicAuth {
	return &http.BasicAuth{
		Username: r.user,
		Password: r.token,
	}
}

func (r *Repo) author() *object.Signature {
	return &object.Signature{
		Name:  r.commitName,
		Email: r.commitEmail,
		When:  time.Now(),
	}
}

func (r *Repo) Clone(ctx context.Context) error {
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

func (r *Repo) Objects() (map[string]string, error) {
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
		content, _ := os.ReadFile(filepath.Join(r.workDir, entry.Name))
		m[entry.Name] = string(content)
	}
	return m, nil
}

func (r *Repo) Open(ctx context.Context) error {
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

func (r *Repo) CloneOrOpen(ctx context.Context) error {
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

func (r *Repo) Pull(ctx context.Context) error {
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

func (r *Repo) Path() string {
	return r.workDir
}

func (r *Repo) Push(ctx context.Context) error {
	if err := r.repo.PushContext(ctx, &git.PushOptions{
		Auth:       r.auth(),
		RemoteName: "origin",
	}); err != nil {
		return fmt.Errorf("failed to push %v: %v", r.branch, err)
	}

	return nil
}

func (r *Repo) Add(path string) error {
	_, err := r.worktree.Add(path)
	return err
}

func (r *Repo) Commit(msg string) error {
	_, err := r.worktree.Commit(msg, &git.CommitOptions{
		Author: r.author(),
	})
	return err
}

func (r *Repo) IsClean() bool {
	status, err := r.worktree.Status()
	if err != nil {
		return false
	}
	return status.IsClean()
}
