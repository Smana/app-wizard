// Assemble the claim object from form state and dump it to YAML (FR-012).
// js-yaml v5 is ESM named-exports only — there is no default export.
import { dump } from "js-yaml";
import type { GVK } from "../api/types";
import { prune } from "./model";

export interface ClaimInput {
  // gvk is derived from the XRD (via /api/schema), never hardcoded (FR-001).
  gvk: GVK;
  name: string;
  namespace?: string;
  spec: unknown;
}

export function buildClaim({ gvk, name, namespace, spec }: ClaimInput): Record<string, unknown> {
  const metadata: Record<string, unknown> = { name: name || "<app-name>" };
  if (namespace) metadata.namespace = namespace;
  return {
    apiVersion: gvk.apiVersion,
    kind: gvk.kind,
    metadata,
    spec: prune(spec) ?? {},
  };
}

export function claimToYaml(input: ClaimInput): string {
  return dump(buildClaim(input), { noRefs: true, lineWidth: 100, sortKeys: false });
}
