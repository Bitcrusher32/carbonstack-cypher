package adversarial

import (
	"path/filepath"
	"sort"
	"strings"
)

const GateFCypherNativeProcessProbeSchema = "carbonstack-cypher-gate-f-native-process-probe/v0"

type GateFNativeProcessFinding struct {
	CaseID             string `json:"case_id"`
	Status             string `json:"status"`
	Severity           string `json:"severity"`
	FindingDisposition string `json:"finding_disposition"`
	Message            string `json:"message"`
}

func GateFCypherNativeProcessProbeCases() []string {
	return []string{
		"ADV-NATIVE-CYPHER-CONFIG-ENV-MISUSE-001",
		"ADV-NATIVE-DB-PATH-CONFUSION-001",
		"ADV-NATIVE-MIGRATION-PATH-CONFUSION-001",
		"ADV-NATIVE-RESTART-SHUTDOWN-BEHAVIOR-001",
		"ADV-NATIVE-STALE-PROCESS-001",
		"ADV-NATIVE-LOG-LEAKAGE-001",
		"ADV-NATIVE-GENERATED-ARTIFACTS-001",
		"ADV-NATIVE-PORT-CONFIG-COLLISION-001",
		"ADV-NATIVE-LOCAL-CLEANUP-HAZARDS-001",
		"ADV-NATIVE-FOREGROUND-PROCESS-FAILURE-PARTIAL-STATE-001",
		"ADV-NATIVE-CLI-TERMINATING-INSPECTION-001",
	}
}

func GateFPathInsideDisposableRoot(runRoot string, candidate string) bool {
	if strings.TrimSpace(runRoot) == "" || strings.TrimSpace(candidate) == "" {
		return false
	}
	rootAbs, err := filepath.Abs(runRoot)
	if err != nil {
		return false
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

func GateFClassifyCypherDBPath(runRoot string, dbPath string) GateFNativeProcessFinding {
	if !GateFPathInsideDisposableRoot(runRoot, dbPath) {
		return GateFNativeProcessFinding{
			CaseID:             "ADV-NATIVE-DB-PATH-CONFUSION-001",
			Status:             "executed_failed",
			Severity:           "high",
			FindingDisposition: "failed",
			Message:            "Cypher DB path is outside disposable run root",
		}
	}
	return GateFNativeProcessFinding{
		CaseID:             "ADV-NATIVE-DB-PATH-CONFUSION-001",
		Status:             "executed_passed",
		Severity:           "informational",
		FindingDisposition: "passed",
		Message:            "Cypher DB path is inside disposable run root",
	}
}

func GateFClassifyCypherPortCollision(portBusy bool) GateFNativeProcessFinding {
	if portBusy {
		return GateFNativeProcessFinding{
			CaseID:             "ADV-NATIVE-PORT-CONFIG-COLLISION-001",
			Status:             "executed_failed",
			Severity:           "high",
			FindingDisposition: "failed",
			Message:            "Cypher selected port is occupied",
		}
	}
	return GateFNativeProcessFinding{
		CaseID:             "ADV-NATIVE-PORT-CONFIG-COLLISION-001",
		Status:             "executed_passed",
		Severity:           "informational",
		FindingDisposition: "passed",
		Message:            "Cypher selected port is available",
	}
}

func GateFRedactCypherConfig(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "private") || strings.Contains(lower, "invite") {
			out[key] = "<redacted>"
			continue
		}
		out[key] = values[key]
	}
	return out
}

func GateFClassifyCypherLogOutput(output string) GateFNativeProcessFinding {
	lower := strings.ToLower(output)
	for _, marker := range []string{"invite token", "secret", "private key", "password", "account_id", "device_id", "relay_space", "db_path"} {
		if strings.Contains(lower, marker) {
			return GateFNativeProcessFinding{
				CaseID:             "ADV-NATIVE-LOG-LEAKAGE-001",
				Status:             "executed_failed",
				Severity:           "release-blocker",
				FindingDisposition: "failed",
				Message:            "Cypher output contains sensitive-enough Gate F marker",
			}
		}
	}
	return GateFNativeProcessFinding{
		CaseID:             "ADV-NATIVE-LOG-LEAKAGE-001",
		Status:             "executed_passed",
		Severity:           "informational",
		FindingDisposition: "passed",
		Message:            "Cypher output does not contain known Gate F sensitive markers",
	}
}
