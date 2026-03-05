package e2b

// GitStatusLabel represents the status of a file in git.
type GitStatusLabel string

const (
	GitStatusModified  GitStatusLabel = "modified"
	GitStatusAdded     GitStatusLabel = "added"
	GitStatusDeleted   GitStatusLabel = "deleted"
	GitStatusRenamed   GitStatusLabel = "renamed"
	GitStatusCopied    GitStatusLabel = "copied"
	GitStatusUntracked GitStatusLabel = "untracked"
	GitStatusIgnored   GitStatusLabel = "ignored"
	GitStatusConflict  GitStatusLabel = "conflict"
	GitStatusUnknown   GitStatusLabel = "unknown"
)

// GitFileStatus represents the status of a single file in git.
type GitFileStatus struct {
	// Name is the file path.
	Name string
	// Status is the human-readable status.
	Status GitStatusLabel
	// IndexStatus is the raw status character for the staging area.
	IndexStatus string
	// WorkingTreeStatus is the raw status character for the working tree.
	WorkingTreeStatus string
	// Staged indicates if the file has staged changes.
	Staged bool
	// RenamedFrom is the original path if the file was renamed.
	RenamedFrom string
}

// GitStatus represents the result of `git status`.
type GitStatus struct {
	// CurrentBranch is the current branch name.
	CurrentBranch string
	// Upstream is the upstream tracking branch.
	Upstream string
	// Ahead is the number of commits ahead of upstream.
	Ahead int
	// Behind is the number of commits behind upstream.
	Behind int
	// Detached is true if HEAD is in detached state.
	Detached bool
	// FileStatus is the list of file statuses.
	FileStatus []GitFileStatus
	// IsClean is true when there are no changes.
	IsClean bool
	// HasChanges is true when there are any changes.
	HasChanges bool
	// HasStaged is true when there are staged changes.
	HasStaged bool
	// HasUntracked is true when there are untracked files.
	HasUntracked bool
	// HasConflicts is true when there are merge conflicts.
	HasConflicts bool
}

// GitBranches represents the result of listing branches.
type GitBranches struct {
	// Current is the current branch name.
	Current string
	// Branches is the list of all branch names.
	Branches []string
}

// GitCloneOpts configures git clone behavior.
type GitCloneOpts struct {
	Path                        string
	Branch                      string
	Depth                       int
	Username                    string
	Password                    string
	DangerouslyStoreCredentials bool
}

// GitInitOpts configures git init behavior.
type GitInitOpts struct {
	Bare          bool
	InitialBranch string
}

// GitAddOpts configures git add behavior.
type GitAddOpts struct {
	Files []string
	All   bool
}

// GitCommitOpts configures git commit behavior.
type GitCommitOpts struct {
	AuthorName  string
	AuthorEmail string
	AllowEmpty  bool
}

// GitResetMode represents the mode for git reset.
type GitResetMode string

const (
	GitResetSoft  GitResetMode = "soft"
	GitResetMixed GitResetMode = "mixed"
	GitResetHard  GitResetMode = "hard"
	GitResetMerge GitResetMode = "merge"
	GitResetKeep  GitResetMode = "keep"
)

// GitResetOpts configures git reset behavior.
type GitResetOpts struct {
	Mode   GitResetMode
	Target string
	Paths  []string
}

// GitRestoreOpts configures git restore behavior.
type GitRestoreOpts struct {
	Paths    []string
	Staged   bool
	Worktree bool
	Source   string
}

// GitPushOpts configures git push behavior.
type GitPushOpts struct {
	Remote   string
	Branch   string
	Username string
	Password string
}

// GitPullOpts configures git pull behavior.
type GitPullOpts struct {
	Remote   string
	Branch   string
	Username string
	Password string
}

// GitDeleteBranchOpts configures git branch deletion.
type GitDeleteBranchOpts struct {
	Force bool
}

// GitConfigScope represents the scope for git config.
type GitConfigScope string

const (
	GitConfigLocal  GitConfigScope = "local"
	GitConfigGlobal GitConfigScope = "global"
	GitConfigSystem GitConfigScope = "system"
)

// GitConfigOpts configures git config behavior.
type GitConfigOpts struct {
	Scope GitConfigScope
}
