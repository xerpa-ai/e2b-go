package e2b

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Git provides git operations that run inside the sandbox.
// All commands use GIT_TERMINAL_PROMPT=0 to prevent interactive prompts.
type Git struct {
	sandbox *Sandbox
}

func newGit(sandbox *Sandbox) *Git {
	return &Git{sandbox: sandbox}
}

func (g *Git) run(ctx context.Context, cmd string, envs ...map[string]string) (*CommandResult, error) {
	env := map[string]string{"GIT_TERMINAL_PROMPT": "0"}
	for _, e := range envs {
		for k, v := range e {
			env[k] = v
		}
	}
	return g.sandbox.Commands.Run(ctx, cmd, WithCommandEnvs(env))
}

func injectAuthURL(rawURL, username, password string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL, err
	}
	if username != "" || password != "" {
		u.User = url.UserPassword(username, password)
	}
	return u.String(), nil
}

// Clone clones a git repository into the sandbox.
//
// Example:
//
//	result, err := sandbox.Git.Clone(ctx, "https://github.com/user/repo.git", nil)
func (g *Git) Clone(ctx context.Context, repoURL string, opts *GitCloneOpts) (*CommandResult, error) {
	cloneURL := repoURL
	if opts != nil && (opts.Username != "" || opts.Password != "") {
		var err error
		cloneURL, err = injectAuthURL(repoURL, opts.Username, opts.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to inject credentials into URL: %w", err)
		}
	}

	args := []string{"git", "clone"}
	if opts != nil {
		if opts.Branch != "" {
			args = append(args, "--branch", opts.Branch)
		}
		if opts.Depth > 0 {
			args = append(args, "--depth", strconv.Itoa(opts.Depth))
		}
	}
	args = append(args, cloneURL)
	if opts != nil && opts.Path != "" {
		args = append(args, opts.Path)
	}

	result, err := g.run(ctx, strings.Join(args, " "))
	if err != nil {
		return nil, err
	}

	if opts != nil && opts.DangerouslyStoreCredentials && opts.Username != "" {
		_, _ = g.run(ctx, "git config --global credential.helper store")
	}

	return result, nil
}

// Init initializes a new git repository.
//
// Example:
//
//	result, err := sandbox.Git.Init(ctx, "/home/user/project", nil)
func (g *Git) Init(ctx context.Context, path string, opts *GitInitOpts) (*CommandResult, error) {
	args := []string{"git", "init"}
	if opts != nil {
		if opts.Bare {
			args = append(args, "--bare")
		}
		if opts.InitialBranch != "" {
			args = append(args, "--initial-branch", opts.InitialBranch)
		}
	}
	args = append(args, path)
	return g.run(ctx, strings.Join(args, " "))
}

// Status returns the parsed git status for a repository.
//
// Example:
//
//	status, err := sandbox.Git.Status(ctx, "/home/user/project")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("Branch:", status.CurrentBranch)
//	fmt.Println("Clean:", status.IsClean)
func (g *Git) Status(ctx context.Context, path string) (*GitStatus, error) {
	result, err := g.run(ctx, fmt.Sprintf("cd %s && git status --porcelain=v1 -b", shellQuote(path)))
	if err != nil {
		return nil, err
	}
	return parseGitStatus(result.Stdout), nil
}

// Branches returns the list of branches in a repository.
//
// Example:
//
//	branches, err := sandbox.Git.Branches(ctx, "/home/user/project")
func (g *Git) Branches(ctx context.Context, path string) (*GitBranches, error) {
	result, err := g.run(ctx, fmt.Sprintf("cd %s && git branch --no-color", shellQuote(path)))
	if err != nil {
		return nil, err
	}
	return parseGitBranches(result.Stdout), nil
}

// CreateBranch creates a new branch.
func (g *Git) CreateBranch(ctx context.Context, path, name string) (*CommandResult, error) {
	return g.run(ctx, fmt.Sprintf("cd %s && git branch %s", shellQuote(path), shellQuote(name)))
}

// CheckoutBranch checks out a branch.
func (g *Git) CheckoutBranch(ctx context.Context, path, name string) (*CommandResult, error) {
	return g.run(ctx, fmt.Sprintf("cd %s && git checkout %s", shellQuote(path), shellQuote(name)))
}

// DeleteBranch deletes a branch.
func (g *Git) DeleteBranch(ctx context.Context, path, name string, opts *GitDeleteBranchOpts) (*CommandResult, error) {
	flag := "-d"
	if opts != nil && opts.Force {
		flag = "-D"
	}
	return g.run(ctx, fmt.Sprintf("cd %s && git branch %s %s", shellQuote(path), flag, shellQuote(name)))
}

// Add stages files for commit.
//
// Example:
//
//	result, err := sandbox.Git.Add(ctx, "/home/user/project", &e2b.GitAddOpts{All: true})
func (g *Git) Add(ctx context.Context, path string, opts *GitAddOpts) (*CommandResult, error) {
	args := []string{"cd", shellQuote(path), "&&", "git", "add"}
	if opts != nil {
		if opts.All {
			args = append(args, "--all")
		} else if len(opts.Files) > 0 {
			for _, f := range opts.Files {
				args = append(args, shellQuote(f))
			}
		} else {
			args = append(args, ".")
		}
	} else {
		args = append(args, ".")
	}
	return g.run(ctx, strings.Join(args, " "))
}

// Commit creates a commit with the given message.
//
// Example:
//
//	result, err := sandbox.Git.Commit(ctx, "/home/user/project", "Initial commit", nil)
func (g *Git) Commit(ctx context.Context, path, message string, opts *GitCommitOpts) (*CommandResult, error) {
	args := []string{"cd", shellQuote(path), "&&", "git", "commit", "-m", shellQuote(message)}
	if opts != nil {
		if opts.AllowEmpty {
			args = append(args, "--allow-empty")
		}
		if opts.AuthorName != "" && opts.AuthorEmail != "" {
			args = append(args, "--author", shellQuote(fmt.Sprintf("%s <%s>", opts.AuthorName, opts.AuthorEmail)))
		}
	}
	return g.run(ctx, strings.Join(args, " "))
}

// Reset performs a git reset.
func (g *Git) Reset(ctx context.Context, path string, opts *GitResetOpts) (*CommandResult, error) {
	args := []string{"cd", shellQuote(path), "&&", "git", "reset"}
	if opts != nil {
		if opts.Mode != "" {
			args = append(args, "--"+string(opts.Mode))
		}
		if opts.Target != "" {
			args = append(args, opts.Target)
		}
		if len(opts.Paths) > 0 {
			args = append(args, "--")
			for _, p := range opts.Paths {
				args = append(args, shellQuote(p))
			}
		}
	}
	return g.run(ctx, strings.Join(args, " "))
}

// Restore restores working tree files.
func (g *Git) Restore(ctx context.Context, path string, opts *GitRestoreOpts) (*CommandResult, error) {
	args := []string{"cd", shellQuote(path), "&&", "git", "restore"}
	if opts != nil {
		if opts.Staged {
			args = append(args, "--staged")
		}
		if opts.Worktree {
			args = append(args, "--worktree")
		}
		if opts.Source != "" {
			args = append(args, "--source", opts.Source)
		}
		if len(opts.Paths) > 0 {
			for _, p := range opts.Paths {
				args = append(args, shellQuote(p))
			}
		}
	}
	return g.run(ctx, strings.Join(args, " "))
}

// Push pushes commits to a remote repository.
func (g *Git) Push(ctx context.Context, path string, opts *GitPushOpts) (*CommandResult, error) {
	envs := map[string]string{}
	args := []string{"cd", shellQuote(path), "&&", "git", "push"}
	if opts != nil {
		if opts.Username != "" || opts.Password != "" {
			envs["GIT_ASKPASS"] = "echo"
			if opts.Password != "" {
				envs["GIT_PASSWORD"] = opts.Password
			}
		}
		if opts.Remote != "" {
			args = append(args, opts.Remote)
		}
		if opts.Branch != "" {
			args = append(args, opts.Branch)
		}
	}
	return g.run(ctx, strings.Join(args, " "), envs)
}

// Pull pulls commits from a remote repository.
func (g *Git) Pull(ctx context.Context, path string, opts *GitPullOpts) (*CommandResult, error) {
	envs := map[string]string{}
	args := []string{"cd", shellQuote(path), "&&", "git", "pull"}
	if opts != nil {
		if opts.Username != "" || opts.Password != "" {
			envs["GIT_ASKPASS"] = "echo"
			if opts.Password != "" {
				envs["GIT_PASSWORD"] = opts.Password
			}
		}
		if opts.Remote != "" {
			args = append(args, opts.Remote)
		}
		if opts.Branch != "" {
			args = append(args, opts.Branch)
		}
	}
	return g.run(ctx, strings.Join(args, " "), envs)
}

// RemoteAdd adds a new remote.
func (g *Git) RemoteAdd(ctx context.Context, path, name, remoteURL string) (*CommandResult, error) {
	return g.run(ctx, fmt.Sprintf("cd %s && git remote add %s %s", shellQuote(path), shellQuote(name), shellQuote(remoteURL)))
}

// RemoteGet returns the URL of a remote.
func (g *Git) RemoteGet(ctx context.Context, path, name string) (string, error) {
	result, err := g.run(ctx, fmt.Sprintf("cd %s && git remote get-url %s", shellQuote(path), shellQuote(name)))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

// SetConfig sets a git config value.
func (g *Git) SetConfig(ctx context.Context, key, value string, opts *GitConfigOpts) (*CommandResult, error) {
	args := []string{"git", "config"}
	if opts != nil && opts.Scope != "" {
		args = append(args, "--"+string(opts.Scope))
	}
	args = append(args, key, shellQuote(value))
	return g.run(ctx, strings.Join(args, " "))
}

// GetConfig gets a git config value.
func (g *Git) GetConfig(ctx context.Context, key string, opts *GitConfigOpts) (string, error) {
	args := []string{"git", "config"}
	if opts != nil && opts.Scope != "" {
		args = append(args, "--"+string(opts.Scope))
	}
	args = append(args, "--get", key)
	result, err := g.run(ctx, strings.Join(args, " "))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

// ConfigureUser sets the git user name and email.
func (g *Git) ConfigureUser(ctx context.Context, name, email string) error {
	if _, err := g.SetConfig(ctx, "user.name", name, &GitConfigOpts{Scope: GitConfigGlobal}); err != nil {
		return err
	}
	_, err := g.SetConfig(ctx, "user.email", email, &GitConfigOpts{Scope: GitConfigGlobal})
	return err
}

// DangerouslyAuthenticate stores git credentials globally using the credential store.
// This persists credentials on disk inside the sandbox.
func (g *Git) DangerouslyAuthenticate(ctx context.Context, username, password string) (*CommandResult, error) {
	if _, err := g.run(ctx, "git config --global credential.helper store"); err != nil {
		return nil, err
	}
	credLine := fmt.Sprintf("https://%s:%s@github.com", url.PathEscape(username), url.PathEscape(password))
	return g.run(ctx, fmt.Sprintf("echo %s >> ~/.git-credentials", shellQuote(credLine)))
}

// shellQuote wraps s in single quotes with proper escaping for shell use.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func parseGitStatus(output string) *GitStatus {
	status := &GitStatus{}
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			parseBranchLine(line[3:], status)
			continue
		}
		if len(line) < 3 {
			continue
		}

		idx := string(line[0])
		wt := string(line[1])
		name := strings.TrimSpace(line[3:])

		fs := GitFileStatus{
			Name:              name,
			IndexStatus:       idx,
			WorkingTreeStatus: wt,
		}

		switch {
		case idx == "?" && wt == "?":
			fs.Status = GitStatusUntracked
			status.HasUntracked = true
		case idx == "!" && wt == "!":
			fs.Status = GitStatusIgnored
		case idx == "U" || wt == "U" || (idx == "A" && wt == "A") || (idx == "D" && wt == "D"):
			fs.Status = GitStatusConflict
			status.HasConflicts = true
		case idx == "R":
			fs.Status = GitStatusRenamed
			fs.Staged = true
			parts := strings.SplitN(name, " -> ", 2)
			if len(parts) == 2 {
				fs.RenamedFrom = parts[0]
				fs.Name = parts[1]
			}
		case idx == "C":
			fs.Status = GitStatusCopied
			fs.Staged = true
		default:
			fs.Status = resolveFileStatus(idx, wt)
			fs.Staged = idx != " " && idx != "?"
		}

		if fs.Staged {
			status.HasStaged = true
		}
		status.FileStatus = append(status.FileStatus, fs)
	}

	status.HasChanges = len(status.FileStatus) > 0
	status.IsClean = !status.HasChanges
	return status
}

func resolveFileStatus(idx, wt string) GitStatusLabel {
	if idx == "M" || wt == "M" {
		return GitStatusModified
	}
	if idx == "A" {
		return GitStatusAdded
	}
	if idx == "D" || wt == "D" {
		return GitStatusDeleted
	}
	return GitStatusUnknown
}

func parseBranchLine(line string, status *GitStatus) {
	if strings.HasPrefix(line, "HEAD (no branch)") || strings.Contains(line, "(HEAD detached") {
		status.Detached = true
		return
	}

	// Format: branch...upstream [ahead N, behind M]
	parts := strings.SplitN(line, "...", 2)
	status.CurrentBranch = strings.TrimSpace(parts[0])
	if len(parts) < 2 {
		return
	}

	rest := parts[1]
	bracketIdx := strings.Index(rest, " [")
	if bracketIdx >= 0 {
		status.Upstream = rest[:bracketIdx]
		info := rest[bracketIdx+2 : len(rest)-1]
		for _, part := range strings.Split(info, ", ") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "ahead ") {
				fmt.Sscanf(part, "ahead %d", &status.Ahead)
			} else if strings.HasPrefix(part, "behind ") {
				fmt.Sscanf(part, "behind %d", &status.Behind)
			}
		}
	} else {
		status.Upstream = strings.TrimSpace(rest)
	}
}

func parseGitBranches(output string) *GitBranches {
	branches := &GitBranches{}
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "* ") {
			name := strings.TrimPrefix(line, "* ")
			branches.Current = name
			branches.Branches = append(branches.Branches, name)
		} else {
			branches.Branches = append(branches.Branches, line)
		}
	}

	return branches
}
