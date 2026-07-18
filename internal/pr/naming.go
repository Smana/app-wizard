package pr

import (
	"fmt"
	"strings"
)

// xplanePrefix is the mandatory name prefix for apps that provision AWS/IAM
// resources. The Crossplane provider-aws credentials are scoped to xplane-*
// (platform constitution: "IAM policy scoped to xplane-* resources"), so an app
// whose composition creates an S3 bucket or a managed SQL instance — each
// backed by an IAM role/policy named after the claim — MUST carry this prefix,
// or those IAM resources fall outside the permission boundary and silently fail
// at runtime (the app deploys, S3/backup access is denied).
const xplanePrefix = "xplane-"

// backendEnabled reports whether spec[key] is an object with `enabled: true`.
func backendEnabled(spec map[string]any, key string) bool {
	b, ok := spec[key].(map[string]any)
	if !ok {
		return false
	}
	enabled, _ := b["enabled"].(bool)
	return enabled
}

// provisionsAWSResources reports whether the spec enables a backend whose
// composition creates AWS/IAM resources named after the claim: an S3 bucket
// (Bucket + EKS Pod Identity role) or a managed SQL instance (CloudNativePG +
// S3 backup IAM).
func provisionsAWSResources(spec map[string]any) bool {
	return backendEnabled(spec, "s3Bucket") || backendEnabled(spec, "sqlInstance")
}

// NamingGate enforces the xplane-* prefix for apps that provision AWS/IAM
// resources. It returns a *GateError to block the PR, or nil when the name is
// acceptable (no AWS-backed feature enabled, or the prefix is already present).
// This is the wizard-side counterpart to the constitution's naming rule: the
// spec-level CEL evaluator binds `self` to the spec only and cannot see
// metadata.name, so the name/spec correlation is enforced here where both the
// app name and the spec are in hand.
func NamingGate(name string, spec map[string]any) *GateError {
	if provisionsAWSResources(spec) && !strings.HasPrefix(name, xplanePrefix) {
		return &GateError{Message: fmt.Sprintf(
			"app %q enables an AWS-backed feature (s3Bucket or sqlInstance) whose IAM resources must be named within the %s* boundary; name it %q instead",
			name, xplanePrefix, xplanePrefix+name)}
	}
	return nil
}
