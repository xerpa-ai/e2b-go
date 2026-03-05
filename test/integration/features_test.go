package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	e2b "github.com/xerpa-ai/e2b-go"
)

func apiKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("E2B_API_KEY")
	if key == "" {
		t.Skip("E2B_API_KEY not set, skipping integration test")
	}
	return key
}

func createSandbox(t *testing.T) *e2b.Sandbox {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	sandbox, err := e2b.NewWithContext(ctx,
		e2b.WithAPIKey(apiKey(t)),
		e2b.WithTimeout(5*time.Minute),
	)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	t.Logf("sandbox created: %s", sandbox.ID)
	return sandbox
}

func TestGitOperations(t *testing.T) {
	sandbox := createSandbox(t)
	defer sandbox.CloseWithContext(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// git init
	t.Log("--- git init ---")
	_, err := sandbox.Git.Init(ctx, "/tmp/testrepo", nil)
	if err != nil {
		t.Fatalf("Git.Init failed: %v", err)
	}

	// configure user
	t.Log("--- git configure user ---")
	err = sandbox.Git.ConfigureUser(ctx, "Test User", "test@example.com")
	if err != nil {
		t.Fatalf("Git.ConfigureUser failed: %v", err)
	}

	// create a file and add
	t.Log("--- create file + git add ---")
	_, err = sandbox.Commands.Run(ctx, "echo 'hello world' > /tmp/testrepo/README.md")
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	_, err = sandbox.Git.Add(ctx, "/tmp/testrepo", &e2b.GitAddOpts{All: true})
	if err != nil {
		t.Fatalf("Git.Add failed: %v", err)
	}

	// status
	t.Log("--- git status ---")
	status, err := sandbox.Git.Status(ctx, "/tmp/testrepo")
	if err != nil {
		t.Fatalf("Git.Status failed: %v", err)
	}
	t.Logf("branch: %s, staged: %v, clean: %v, files: %d",
		status.CurrentBranch, status.HasStaged, status.IsClean, len(status.FileStatus))

	if !status.HasStaged {
		t.Error("expected staged changes after git add")
	}

	// commit
	t.Log("--- git commit ---")
	_, err = sandbox.Git.Commit(ctx, "/tmp/testrepo", "initial commit", nil)
	if err != nil {
		t.Fatalf("Git.Commit failed: %v", err)
	}

	// status after commit should be clean
	status, err = sandbox.Git.Status(ctx, "/tmp/testrepo")
	if err != nil {
		t.Fatalf("Git.Status after commit failed: %v", err)
	}
	if !status.IsClean {
		t.Errorf("expected clean repo after commit, got %d changed files", len(status.FileStatus))
	}
	t.Logf("post-commit: branch=%s, clean=%v", status.CurrentBranch, status.IsClean)

	// branches
	t.Log("--- git branches ---")
	branches, err := sandbox.Git.Branches(ctx, "/tmp/testrepo")
	if err != nil {
		t.Fatalf("Git.Branches failed: %v", err)
	}
	t.Logf("branches: %v, current: %s", branches.Branches, branches.Current)
	if len(branches.Branches) == 0 {
		t.Error("expected at least one branch")
	}

	// create + checkout branch
	t.Log("--- create + checkout branch ---")
	_, err = sandbox.Git.CreateBranch(ctx, "/tmp/testrepo", "feature-x")
	if err != nil {
		t.Fatalf("Git.CreateBranch failed: %v", err)
	}
	_, err = sandbox.Git.CheckoutBranch(ctx, "/tmp/testrepo", "feature-x")
	if err != nil {
		t.Fatalf("Git.CheckoutBranch failed: %v", err)
	}

	status, err = sandbox.Git.Status(ctx, "/tmp/testrepo")
	if err != nil {
		t.Fatalf("Git.Status on feature branch failed: %v", err)
	}
	if status.CurrentBranch != "feature-x" {
		t.Errorf("expected branch feature-x, got %s", status.CurrentBranch)
	}
	t.Logf("now on branch: %s", status.CurrentBranch)

	// git config
	t.Log("--- git config ---")
	name, err := sandbox.Git.GetConfig(ctx, "user.name", &e2b.GitConfigOpts{Scope: e2b.GitConfigGlobal})
	if err != nil {
		t.Fatalf("Git.GetConfig failed: %v", err)
	}
	if name != "Test User" {
		t.Errorf("expected user.name 'Test User', got %q", name)
	}
	t.Logf("user.name = %s", name)

	t.Log("=== Git operations: ALL PASSED ===")
}

func TestPauseResume(t *testing.T) {
	sandbox := createSandbox(t)
	sandboxID := sandbox.ID
	key := apiKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// write a marker file so we can verify state survives pause/resume
	t.Log("--- writing marker file ---")
	_, err := sandbox.Commands.Run(ctx, "echo 'pause-test-marker' > /tmp/marker.txt")
	if err != nil {
		t.Fatalf("failed to write marker: %v", err)
	}

	// pause
	t.Log("--- pausing sandbox ---")
	err = sandbox.Pause(ctx)
	if err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	t.Logf("sandbox %s paused", sandboxID)

	// wait a moment for the pause to take effect
	time.Sleep(3 * time.Second)

	// resume by connecting
	t.Log("--- resuming sandbox via Connect ---")
	resumed, err := e2b.ConnectWithContext(ctx, sandboxID,
		e2b.WithAPIKey(key),
		e2b.WithTimeout(5*time.Minute),
	)
	if err != nil {
		t.Fatalf("ConnectWithContext (resume) failed: %v", err)
	}
	defer resumed.CloseWithContext(context.Background())
	t.Logf("sandbox %s resumed", resumed.ID)

	// verify marker file survived
	t.Log("--- verifying marker file ---")
	result, err := resumed.Commands.Run(ctx, "cat /tmp/marker.txt")
	if err != nil {
		t.Fatalf("failed to read marker after resume: %v", err)
	}
	content := strings.TrimSpace(result.Stdout)
	if content != "pause-test-marker" {
		t.Errorf("marker content = %q, want 'pause-test-marker'", content)
	}
	t.Logf("marker content: %s", content)

	t.Log("=== Pause/Resume: ALL PASSED ===")
}

func TestSnapshot(t *testing.T) {
	sandbox := createSandbox(t)
	defer sandbox.CloseWithContext(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// write something unique so we can identify the snapshot
	marker := fmt.Sprintf("snapshot-marker-%d", time.Now().UnixNano())
	t.Logf("--- writing marker: %s ---", marker)
	_, err := sandbox.Commands.Run(ctx, fmt.Sprintf("echo '%s' > /tmp/snap-marker.txt", marker))
	if err != nil {
		t.Fatalf("failed to write marker: %v", err)
	}

	// create snapshot
	t.Log("--- creating snapshot ---")
	snapInfo, err := sandbox.CreateSnapshot(ctx)
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	t.Logf("snapshot created: %s", snapInfo.SnapshotID)

	if snapInfo.SnapshotID == "" {
		t.Fatal("snapshot ID is empty")
	}

	// create a new sandbox from the snapshot
	t.Log("--- creating sandbox from snapshot ---")
	sandbox2, err := e2b.NewWithContext(ctx,
		e2b.WithAPIKey(apiKey(t)),
		e2b.WithTemplate(snapInfo.SnapshotID),
		e2b.WithTimeout(5*time.Minute),
	)
	if err != nil {
		t.Fatalf("failed to create sandbox from snapshot: %v", err)
	}
	defer sandbox2.CloseWithContext(context.Background())
	t.Logf("sandbox from snapshot: %s", sandbox2.ID)

	// verify the marker file exists in the new sandbox
	t.Log("--- verifying marker in snapshot sandbox ---")
	result, err := sandbox2.Commands.Run(ctx, "cat /tmp/snap-marker.txt")
	if err != nil {
		t.Fatalf("failed to read marker from snapshot sandbox: %v", err)
	}
	content := strings.TrimSpace(result.Stdout)
	if content != marker {
		t.Errorf("marker = %q, want %q", content, marker)
	}
	t.Logf("marker verified: %s", content)

	t.Log("=== Snapshot: ALL PASSED ===")
}

func TestLifecycle(t *testing.T) {
	key := apiKey(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// create sandbox with lifecycle config
	t.Log("--- creating sandbox with lifecycle onTimeout=pause ---")
	sandbox, err := e2b.NewWithContext(ctx,
		e2b.WithAPIKey(key),
		e2b.WithTimeout(5*time.Minute),
		e2b.WithLifecycle(e2b.SandboxLifecycle{
			OnTimeout:  "pause",
			AutoResume: true,
		}),
	)
	if err != nil {
		t.Fatalf("failed to create sandbox with lifecycle: %v", err)
	}
	defer sandbox.CloseWithContext(context.Background())
	t.Logf("sandbox created with lifecycle: %s", sandbox.ID)

	// verify it's running
	running, err := sandbox.IsRunning(ctx)
	if err != nil {
		t.Fatalf("IsRunning failed: %v", err)
	}
	if !running {
		t.Error("expected sandbox to be running")
	}
	t.Logf("sandbox running: %v", running)

	t.Log("=== Lifecycle: ALL PASSED ===")
}

func TestSandboxLogs(t *testing.T) {
	sandbox := createSandbox(t)
	defer sandbox.CloseWithContext(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// generate some output
	_, err := sandbox.Commands.Run(ctx, "echo 'log test line'")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// small delay for logs to become available
	time.Sleep(2 * time.Second)

	// fetch logs
	t.Log("--- fetching sandbox logs ---")
	logs, err := sandbox.GetLogs(ctx, e2b.WithLogsLogLimit(50))
	if err != nil {
		t.Fatalf("GetLogs failed: %v", err)
	}
	t.Logf("received %d log entries", len(logs))
	for i, entry := range logs {
		if i >= 5 {
			t.Logf("  ... and %d more", len(logs)-5)
			break
		}
		t.Logf("  [%s] %s: %s", entry.Timestamp, entry.Level, entry.Message)
	}

	t.Log("=== Sandbox Logs: ALL PASSED ===")
}
