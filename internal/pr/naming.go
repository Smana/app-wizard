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

// awsBackedBackends are the spec backends whose composition creates IAM
// resources named after the claim (S3 bucket + EKS Pod Identity role;
// CloudNativePG + S3 backup IAM). Keep in sync with the App composition: adding
// an AWS-backed backend here is what extends the xplane-* naming guard to it.
var awsBackedBackends = []string{"s3Bucket", "sqlInstance"}

// provisionsAWSResources reports whether the spec enables any AWS-backed backend
// (see awsBackedBackends).
func provisionsAWSResources(spec map[string]any) bool {
	for _, backend := range awsBackedBackends {
		if backendEnabled(spec, backend) {
			return true
		}
	}
	return false
}

// NamingGate enforces the xplane-* prefix for apps that provision AWS/IAM
// resources. It returns a *GateError to block the PR, or nil when the name is
// acceptable (no AWS-backed feature enabled, or the prefix is already present).
//
// This is the wizard's fast-feedback layer: the wizard's schema pipeline
// evaluates CEL with `self` bound to the spec only (mirroring Kubernetes'
// spec-scoped rules), so it cannot correlate metadata.name with the spec — the
// check lives in Go, where both the app name and the spec are in hand.
// Platform-wide enforcement (kubectl / Flux paths that bypass the wizard)
// belongs in a root-level XRD x-kubernetes-validations rule, which CAN read
// self.metadata.name.
func NamingGate(name string, spec map[string]any) *GateError {
	if provisionsAWSResources(spec) && !strings.HasPrefix(name, xplanePrefix) {
		return &GateError{Message: fmt.Sprintf(
			"app %q enables an AWS-backed feature (%s) whose IAM resources must be named within the %s* boundary; name it %q instead",
			name, strings.Join(awsBackedBackends, ", "), xplanePrefix, xplanePrefix+name)}
	}
	return nil
}
