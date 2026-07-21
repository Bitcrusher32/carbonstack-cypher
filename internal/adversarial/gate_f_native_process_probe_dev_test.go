package adversarial

import (
	"path/filepath"
	"testing"
)

func TestGateFCypherNativeProcessProbeCases(t *testing.T) {
	if len(GateFCypherNativeProcessProbeCases()) != 11 {
		t.Fatalf("expected 11 Cypher Gate F probe cases")
	}
}

func TestGateFCypherDBPathMustStayInsideDisposableRoot(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "cypher.db")
	if got := GateFClassifyCypherDBPath(root, inside); got.Status != "executed_passed" {
		t.Fatalf("inside DB should pass: %#v", got)
	}
	outside := filepath.Join(filepath.Dir(root), "cypher.db")
	if got := GateFClassifyCypherDBPath(root, outside); got.Status != "executed_failed" || got.Severity != "high" {
		t.Fatalf("outside DB should fail high: %#v", got)
	}
}

func TestGateFCypherPortCollision(t *testing.T) {
	if got := GateFClassifyCypherPortCollision(false); got.Status != "executed_passed" {
		t.Fatalf("free port should pass: %#v", got)
	}
	if got := GateFClassifyCypherPortCollision(true); got.Status != "executed_failed" || got.Severity != "high" {
		t.Fatalf("busy port should fail high: %#v", got)
	}
}

func TestGateFCypherConfigRedaction(t *testing.T) {
	redacted := GateFRedactCypherConfig(map[string]string{
		"addr":         "127.0.0.1:8080",
		"invite_token": "abc",
		"secret":       "def",
	})
	if redacted["addr"] != "127.0.0.1:8080" {
		t.Fatalf("non-sensitive addr changed")
	}
	if redacted["invite_token"] != "<redacted>" || redacted["secret"] != "<redacted>" {
		t.Fatalf("sensitive config values were not redacted: %#v", redacted)
	}
}

func TestGateFCypherLogSensitiveMarkers(t *testing.T) {
	if got := GateFClassifyCypherLogOutput("config ok addr=127.0.0.1:8080"); got.Status != "executed_passed" {
		t.Fatalf("safe output should pass: %#v", got)
	}
	if got := GateFClassifyCypherLogOutput("db_path=/tmp/cypher.db invite token leaked"); got.Status != "executed_failed" || got.Severity != "release-blocker" {
		t.Fatalf("sensitive output should fail release-blocker: %#v", got)
	}
}
