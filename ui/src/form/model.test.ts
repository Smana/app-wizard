import { describe, expect, it } from "vitest";
import { fixtureSchema } from "../api/__fixtures__/schema";
import {
  buildLayout,
  clearInvalidForType,
  fieldVisibleForType,
  hintFor,
  typeGatesFromCEL,
} from "./model";

describe("layout tiering (SC-002 + progressive disclosure)", () => {
  const layout = buildLayout(fixtureSchema);

  it("puts every schema property somewhere — none are dropped", () => {
    const placed = new Set<string>([
      ...layout.basic.map((f) => f.key),
      ...layout.groups.flatMap((g) => g.fields.map((f) => f.key)),
      ...layout.ungrouped.map((f) => f.key),
    ]);
    const schemaKeys = Object.keys(
      (fixtureSchema.jsonSchema as { properties: Record<string, unknown> }).properties,
    );
    for (const k of schemaKeys) expect(placed.has(k)).toBe(true);
  });

  it("SC-002: a field present in jsonSchema but ABSENT from hints defaults to advanced tier", () => {
    // futureScalarField exists in the schema fixture, not in hints.fields.
    expect(hintFor("futureScalarField", fixtureSchema.hints).tier).toBe("advanced");
    const inBasic = layout.basic.some((f) => f.key === "futureScalarField");
    const inAdvanced =
      layout.groups.some((g) => g.fields.some((f) => f.key === "futureScalarField")) ||
      layout.ungrouped.some((f) => f.key === "futureScalarField");
    expect(inBasic).toBe(false);
    expect(inAdvanced).toBe(true);
  });
});

// The workload-type gate is parsed out of the XRD's x-kubernetes-validations
// rather than mirrored in a hand-written table. These are the ACTUAL rules the
// platform App XRD ships (Smana/crossplane-configuration apis/app/definition.yaml),
// pasted verbatim: if the parser stops understanding the shape an XRD writes,
// these fail rather than the form silently hiding — and deleting — a valid field.
const APP_XRD_RULES = [
  {
    rule: "!has(self.autoscaling) || !self.autoscaling.enabled || self.autoscaling.minReplicas <= self.autoscaling.maxReplicas",
    message: "autoscaling.minReplicas must be <= maxReplicas",
  },
  {
    rule: "!has(self.route) || !self.route.enabled || has(self.route.hostname)",
    message: "route.hostname is required when route is enabled",
  },
  {
    rule: "!has(self.objectStore) || self.objectStore.permissions != 'custom' || (has(self.objectStore.aws) && has(self.objectStore.aws.customPolicy))",
    message: "objectStore.aws.customPolicy is required when objectStore.permissions is custom",
  },
  {
    rule: "!has(self.type) || self.type != 'cron' || has(self.schedule)",
    message: "schedule is required when type is 'cron'",
  },
  {
    rule: "!has(self.schedule) || (has(self.type) && self.type == 'cron')",
    message: "schedule is only valid when type is 'cron'",
  },
  {
    rule: "!has(self.route) || !self.route.enabled || !has(self.type) || self.type == 'web'",
    message: "route is only valid when type is 'web'",
  },
  {
    rule: "!has(self.gateway) || !self.gateway.enabled || !has(self.type) || self.type == 'web'",
    message: "gateway is only valid when type is 'web'",
  },
  {
    rule: "!has(self.autoscaling) || !self.autoscaling.enabled || !has(self.type) || self.type != 'cron'",
    message: "autoscaling is not valid when type is 'cron'",
  },
  {
    rule: "!has(self.pdb) || !self.pdb.enabled || !has(self.type) || self.type != 'cron'",
    message: "pdb is not valid when type is 'cron'",
  },
];

describe("workload-type gates derived from CEL", () => {
  const gates = typeGatesFromCEL(APP_XRD_RULES);
  const visible = (key: string, type: string) => fieldVisibleForType(key, type, gates);

  it("reads 'only valid when type is web' as web-only", () => {
    for (const key of ["route", "gateway"]) {
      expect(visible(key, "web")).toBe(true);
      expect(visible(key, "worker")).toBe(false);
      expect(visible(key, "cron")).toBe(false);
    }
  });

  it("reads 'not valid when type is cron' as everything-but-cron", () => {
    for (const key of ["autoscaling", "pdb"]) {
      expect(visible(key, "web")).toBe(true);
      expect(visible(key, "worker")).toBe(true);
      expect(visible(key, "cron")).toBe(false);
    }
  });

  it("reads 'only valid when type is cron' as cron-only", () => {
    expect(visible("schedule", "cron")).toBe(true);
    expect(visible("schedule", "web")).toBe(false);
  });

  it("leaves fields no rule constrains visible for every type", () => {
    // `service` and `cron` were hidden for non-web by the old hardcoded table,
    // and clearInvalidForType then deleted whatever had been typed there — but
    // the XRD never restricted either. Regression guard.
    for (const type of ["web", "worker", "cron"]) {
      expect(visible("service", type)).toBe(true);
      expect(visible("cron", type)).toBe(true);
      expect(visible("image", type)).toBe(true);
      expect(visible("objectStore", type)).toBe(true);
    }
  });

  it("ignores requirement rules that guard self.type itself", () => {
    // "schedule is required when type is 'cron'" guards self.type, not a field.
    expect(gates.has("type")).toBe(false);
  });

  it("clearInvalidForType strips only what the gates actually forbid", () => {
    const spec = { route: { enabled: true }, service: { port: 8080 }, schedule: "0 3 * * *" };
    expect(clearInvalidForType(spec, "cron", gates)).toEqual({
      service: { port: 8080 },
      schedule: "0 3 * * *",
    });
  });

  it("returns the same reference when nothing needs clearing (no render loop)", () => {
    const spec = { service: { port: 8080 } };
    expect(clearInvalidForType(spec, "worker", gates)).toBe(spec);
  });
});

describe("enabled-guarded gates (regression)", () => {
  const gates = typeGatesFromCEL(APP_XRD_RULES);

  it("keeps a DISABLED gated block on a workload type that forbids it", () => {
    // "route is only valid when type is 'web'" is really "an ENABLED route is".
    // A committed `route: {enabled:false}` on a worker is legal CEL, so loading
    // that app for edit must not strip it — doing so made the update PR delete
    // the block from Git.
    const spec = {
      route: { enabled: false, hostname: "keepme" },
      autoscaling: { enabled: false, minReplicas: 3 },
    };
    expect(clearInvalidForType(spec, "worker", gates)).toBe(spec);
    expect(clearInvalidForType(spec, "cron", gates)).toBe(spec);
  });

  it("still clears an ENABLED block that the type forbids", () => {
    const spec = { route: { enabled: true, hostname: "api.example.com" } };
    expect(clearInvalidForType(spec, "worker", gates)).toEqual({});
  });

  it("records the enabled guard only for rules that carry one", () => {
    expect(gates.get("route")?.enabledGuarded).toBe(true);
    expect(gates.get("autoscaling")?.enabledGuarded).toBe(true);
    // schedule's rule has no `!self.schedule.enabled` clause — it is unconditional.
    expect(gates.get("schedule")?.enabledGuarded).toBe(false);
  });

  it("clears an unconditionally-gated field regardless of any enabled flag", () => {
    const spec = { schedule: "0 3 * * *" };
    expect(clearInvalidForType(spec, "web", gates)).toEqual({});
  });
});
