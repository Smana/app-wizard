// Pure helpers that turn a SchemaPayload into an ordered, tiered field layout
// and provide immutable get/set on the nested spec object by dot/array path.
import type { CELRule, FieldHint, GroupHint, SchemaPayload, Tier } from "../api/types";
import { asSchema, type JSONSchema } from "./jsonSchema";

export interface TopField {
  key: string;
  schema: JSONSchema;
  hint: FieldHint; // synthesised default when absent from hints
  order: number;
}

// A field with no hint defaults to the "advanced" tier and is NEVER dropped.
// This is load-bearing for SC-002 (new XRD field appears automatically).
export function hintFor(key: string, hints: SchemaPayload["hints"]): FieldHint {
  return hints.fields[key] ?? { tier: "advanced" };
}

export interface Layout {
  basic: TopField[];
  // Basic-tier fields organised into always-open display blocks by their group
  // hint (same GroupHint defs as the advanced groups), groups themselves ordered.
  // Basic fields whose hint.group is unknown/missing land in a trailing
  // "Details" block so nothing is dropped. This drives the first-screen sections.
  basicGroups: Array<{ group: GroupHint; fields: TopField[] }>;
  // Advanced/expert fields organised by group, groups themselves ordered.
  groups: Array<{ group: GroupHint; fields: TopField[] }>;
  // Advanced/expert fields whose hint.group is unknown/missing land here so they
  // are still rendered (never dropped).
  ungrouped: TopField[];
}

const FALLBACK_GROUP: GroupHint = {
  id: "__more__",
  label: "More options",
  tier: "advanced",
  order: 999,
};

// Trailing block for basic fields that carry no (known) group hint — keeps the
// first screen complete without inventing a group. Ordered last.
const BASIC_FALLBACK_GROUP: GroupHint = {
  id: "__basic_details__",
  label: "Details",
  tier: "basic",
  order: 999,
};

export function buildLayout(payload: SchemaPayload): Layout {
  const props = asSchema(payload.jsonSchema).properties ?? {};
  const all: TopField[] = Object.entries(props).map(([key, schema]) => {
    const hint = hintFor(key, payload.hints);
    return { key, schema, hint, order: hint.order ?? 500 };
  });

  const byOrder = (a: TopField, b: TopField) =>
    a.order - b.order || a.key.localeCompare(b.key);

  const basic = all.filter((f) => f.hint.tier === "basic").sort(byOrder);
  const rest = all.filter((f) => f.hint.tier !== "basic");

  const groupsById = new Map<string, GroupHint>();
  for (const g of payload.hints.groups) groupsById.set(g.id, g);

  // --- Basic tier: grouped into always-open display blocks. Fields with no
  // known group fall into a trailing "Details" block (never dropped). ---
  const basicGroups = groupFields(basic, groupsById, BASIC_FALLBACK_GROUP, byOrder);

  const grouped = new Map<string, TopField[]>();
  const ungrouped: TopField[] = [];
  for (const f of rest) {
    const gid = f.hint.group;
    if (gid && groupsById.has(gid)) {
      const arr = grouped.get(gid) ?? [];
      arr.push(f);
      grouped.set(gid, arr);
    } else {
      ungrouped.push(f);
    }
  }

  const groups = [...grouped.entries()]
    .map(([gid, fields]) => ({
      group: groupsById.get(gid)!,
      fields: fields.sort(byOrder),
    }))
    .sort((a, b) => a.group.order - b.group.order);

  return { basic, basicGroups, groups, ungrouped: ungrouped.sort(byOrder) };
}

// Partition a set of fields into ordered { group, fields } blocks, keyed by
// hint.group; fields whose group is unknown/absent collect into `fallback` so
// none are dropped. The fallback block only appears when it has fields.
function groupFields(
  fields: TopField[],
  groupsById: Map<string, GroupHint>,
  fallback: GroupHint,
  byOrder: (a: TopField, b: TopField) => number,
): Array<{ group: GroupHint; fields: TopField[] }> {
  const grouped = new Map<string, TopField[]>();
  for (const f of fields) {
    const gid = f.hint.group && groupsById.has(f.hint.group) ? f.hint.group : fallback.id;
    const arr = grouped.get(gid) ?? [];
    arr.push(f);
    grouped.set(gid, arr);
  }
  return [...grouped.entries()]
    .map(([gid, fs]) => ({
      group: gid === fallback.id ? fallback : groupsById.get(gid)!,
      fields: fs.sort(byOrder),
    }))
    .sort((a, b) => a.group.order - b.group.order);
}

export const fallbackGroup = FALLBACK_GROUP;

// --- Immutable nested get/set by path segments -----------------------------

export type PathSeg = string | number;

export function getAt(obj: unknown, path: PathSeg[]): unknown {
  let cur: unknown = obj;
  for (const seg of path) {
    if (cur == null) return undefined;
    cur = (cur as Record<PathSeg, unknown>)[seg];
  }
  return cur;
}

export function setAt(obj: unknown, path: PathSeg[], value: unknown): unknown {
  if (path.length === 0) return value;
  const [head, ...tail] = path;
  if (typeof head === "number") {
    const arr = Array.isArray(obj) ? [...obj] : [];
    arr[head] = setAt(arr[head], tail, value);
    return arr;
  }
  const base = obj && typeof obj === "object" ? { ...(obj as object) } : {};
  (base as Record<string, unknown>)[head] = setAt(
    (base as Record<string, unknown>)[head],
    tail,
    value,
  );
  return base;
}

export function deleteAt(obj: unknown, path: PathSeg[]): unknown {
  if (path.length === 0) return undefined;
  const [head, ...tail] = path;
  if (tail.length === 0) {
    if (typeof head === "number" && Array.isArray(obj)) {
      return obj.filter((_, i) => i !== head);
    }
    if (obj && typeof obj === "object") {
      const copy = { ...(obj as Record<string, unknown>) };
      delete copy[head as string];
      return copy;
    }
    return obj;
  }
  const child = getAt(obj, [head]);
  return setAt(obj, [head], deleteAt(child, tail));
}

export function pathToString(path: PathSeg[]): string {
  return path
    .map((s) => (typeof s === "number" ? `[${s}]` : s))
    .join(".")
    .replace(/\.\[/g, "[");
}

// Recursively strip undefined / empty-object / empty-array so the YAML pane and
// claim stay tidy.
export function prune(value: unknown): unknown {
  if (Array.isArray(value)) {
    const arr = value.map(prune).filter((v) => v !== undefined);
    // Drop empty arrays entirely so defaulted/untouched list fields (e.g.
    // route.rules) never appear in the generated claim.
    return arr.length ? arr : undefined;
  }
  if (value && typeof value === "object") {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      const pv = prune(v);
      if (pv === undefined) continue;
      if (pv && typeof pv === "object" && !Array.isArray(pv) && Object.keys(pv).length === 0)
        continue;
      out[k] = pv;
    }
    return out;
  }
  if (value === "" || value === undefined || value === null) return undefined;
  return value;
}

// --- Workload-type gating, derived from the XRD's own CEL rules ------------
//
// Some top-level fields only apply to certain workload types ("route is only
// valid when type is 'web'"). That knowledge belongs to the XRD, which states it
// as x-kubernetes-validations — and the backend already ships those rules in the
// schema payload. This used to be a hand-written table of key→type mirroring
// them, which is a drift hazard by construction: the XRD lives in a different
// repository, so the table silently went stale (it hid `service` and `cron` for
// non-web workloads even though no rule ever restricted them, and
// clearInvalidForType then DELETED what the user had typed there).
//
// So parse the gate out of the rules instead. Two shapes cover what an XRD
// writes, both guarded by `!has(self.<key>)`:
//
//   !has(self.route) || … || self.type == 'web'    → route is web-only
//   !has(self.pdb)   || … || self.type != 'cron'   → pdb is anything-but-cron
//
// Anything unrecognised leaves the field ungated — fail OPEN on purpose. An
// over-permissive form shows a field the backend then rejects with a readable
// CEL message; an over-restrictive one silently discards the user's input, which
// is the failure this replaced.
export interface TypeGate {
  allow: Set<string>;
  deny: Set<string>;
}

const GUARDED_KEY = /!has\(self\.([A-Za-z0-9_]+)\)/;
const TYPE_EQ = /self\.type\s*==\s*'([^']+)'/g;
const TYPE_NE = /self\.type\s*!=\s*'([^']+)'/g;

export function typeGatesFromCEL(rules: CELRule[]): Map<string, TypeGate> {
  const gates = new Map<string, TypeGate>();
  for (const { rule } of rules) {
    if (!rule || !rule.includes("self.type")) continue;

    // The gated field is whatever the rule guards with `!has(...)` first. A rule
    // that guards `self.type` itself is a requirement ("schedule is required
    // when type is 'cron'"), not a visibility gate — skip it.
    const key = GUARDED_KEY.exec(rule)?.[1];
    if (!key || key === "type") continue;

    const gate = gates.get(key) ?? { allow: new Set<string>(), deny: new Set<string>() };
    for (const m of rule.matchAll(TYPE_EQ)) gate.allow.add(m[1]);
    for (const m of rule.matchAll(TYPE_NE)) gate.deny.add(m[1]);
    if (gate.allow.size > 0 || gate.deny.size > 0) gates.set(key, gate);
  }
  return gates;
}

// Whether a top-level key applies to `type`. Both the form (hide the field) and
// clearInvalidForType (strip the stale value) go through here, so the two can
// never disagree.
export function fieldVisibleForType(
  key: string,
  type: string,
  gates: Map<string, TypeGate>,
): boolean {
  const gate = gates.get(key);
  if (!gate) return true;
  if (gate.deny.has(type)) return false;
  if (gate.allow.size > 0 && !gate.allow.has(type)) return false;
  return true;
}

// Clear now-invalid top-level spec keys when the workload type changes. This
// prevents a hidden field from leaving a stale value that trips CEL validation
// and blocks the PR.
//
// CRITICAL: returns the SAME object reference when nothing needs deleting, so the
// caller's effect + setSpec can't loop forever. We check key presence first and
// only rebuild when a key is actually present.
export function clearInvalidForType(
  spec: unknown,
  type: string,
  gates: Map<string, TypeGate>,
): unknown {
  const present = [...gates.keys()].filter(
    (k) => !fieldVisibleForType(k, type, gates) && getAt(spec, [k]) !== undefined,
  );
  if (present.length === 0) return spec; // no-op: preserve reference (no render loop)

  let next = spec;
  for (const k of present) next = deleteAt(next, [k]);
  return next;
}

// Tier badge helper.
export function tierBadgeVariant(tier: Tier): "secondary" | "outline" | "default" {
  if (tier === "expert") return "outline";
  if (tier === "advanced") return "secondary";
  return "default";
}
