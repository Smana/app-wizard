// Recursive, schema-driven field widget. Given a JSONSchema node + its path in
// the spec, it renders the right control and recurses for objects/arrays. It is
// entirely generic — no field is hardcoded (FR-001 / SC-002).
import { useEffect, useMemo, useRef, useState } from "react";
import type { FieldError, UIHints } from "../api/types";
import { Alert, AlertDescription } from "../components/ui/alert";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Input, Textarea } from "../components/ui/input";
import { Select } from "../components/ui/select";
import { Switch } from "../components/ui/collapsible";
import { XIcon } from "../components/ui/icons";
import { asSchema, type JSONSchema } from "./jsonSchema";
import { deleteAt, getAt, pathToString, setAt, type PathSeg } from "./model";
import { errorMatchesPath } from "./validation";

interface FieldProps {
  schema: JSONSchema;
  path: PathSeg[];
  spec: unknown;
  onChange: (next: unknown) => void;
  errors: FieldError[];
  label?: string;
  help?: string;
  placeholder?: string;
  labelledById?: string;
  // Presentation overlay + basic-screen filtering (FIX 3). When basicScreen is
  // true, object children are limited to basic-tier or required keys; when
  // false/undefined, ALL children render (advanced/expert groups).
  hints?: UIHints;
  basicScreen?: boolean;
}

function Label({ id, children }: { id?: string; children: React.ReactNode }) {
  return (
    <label htmlFor={id} className="text-sm font-medium">
      {children}
    </label>
  );
}

function Help({ text }: { text?: string }) {
  if (!text) return null;
  return <p className="text-xs text-muted-foreground">{text}</p>;
}

function fieldErrors(errors: FieldError[], path: PathSeg[]): FieldError[] {
  const s = pathToString(path);
  return errors.filter((e) => errorMatchesPath(e.path, s));
}

function humanize(key: string): string {
  return key
    .replace(/([A-Z])/g, " $1")
    .replace(/^./, (c) => c.toUpperCase())
    .trim();
}

export function Field({
  schema,
  path,
  spec,
  onChange,
  errors,
  label,
  help,
  placeholder,
  labelledById,
  hints,
  basicScreen,
}: FieldProps) {
  const s = asSchema(schema);
  const value = getAt(spec, path);
  const id = labelledById ?? `f-${pathToString(path)}`;
  const errs = fieldErrors(errors, path);
  const help_ = help ?? s.description;
  // Compile the string-field pattern once per pattern (used in the default/string
  // branch below). Hook must be called unconditionally, before any early return.
  const patternRe = useMemo(() => (s.pattern ? new RegExp(s.pattern) : null), [s.pattern]);

  const errorNode =
    errs.length > 0 ? (
      <ul className="text-xs text-destructive">
        {errs.map((e, i) => (
          <li key={i}>{e.message}</li>
        ))}
      </ul>
    ) : null;

  // enum → select. Do NOT pre-select the schema default: an unset field shows the
  // "— select —" option so the form never looks pre-filled (FIX 1). When unset and
  // a default exists, surface it in the help text instead of writing it to state.
  if (s.enum && s.enum.length > 0) {
    const isUnset = value === undefined || value === null || value === "";
    const enumHelp =
      isUnset && s.default != null
        ? [help_, `Default: ${String(s.default)}`].filter(Boolean).join(" ")
        : help_;
    return (
      <div className="space-y-1">
        {label && <Label id={id}>{label}</Label>}
        <Select
          id={id}
          value={String(value ?? "")}
          onChange={(e) => onChange(setAt(spec, path, e.target.value || undefined))}
        >
          <option value="">— select —</option>
          {s.enum.map((opt) => (
            <option key={String(opt)} value={String(opt)}>
              {String(opt)}
            </option>
          ))}
        </Select>
        <Help text={enumHelp} />
        {errorNode}
      </div>
    );
  }

  switch (s.type) {
    case "boolean": {
      const boolChecked = Boolean(value ?? s.default ?? false);
      // Generic public-exposure guardrail: any boolean whose path ends with
      // `internetFacing` shows a warning when toggled ON (FIX 2).
      const isInternetFacing = path[path.length - 1] === "internetFacing";
      return (
        <div className="space-y-2">
          <div className="flex items-start justify-between gap-3">
            <div className="space-y-0.5">
              {label && <Label id={id}>{label}</Label>}
              <Help text={help_} />
            </div>
            <Switch
              id={id}
              checked={boolChecked}
              onCheckedChange={(v) => onChange(setAt(spec, path, v))}
            />
          </div>
          {isInternetFacing && boolChecked && (
            <Alert variant="warning">
              <AlertDescription>
                ⚠ Public exposure — this serves the app on the public internet.
                Use a private route unless public access is required.
              </AlertDescription>
            </Alert>
          )}
          {errorNode}
        </div>
      );
    }

    case "integer":
    case "number":
      return (
        <div className="space-y-1">
          {label && <Label id={id}>{label}</Label>}
          <Input
            id={id}
            type="number"
            inputMode="numeric"
            min={s.minimum}
            max={s.maximum}
            placeholder={placeholder ?? (s.default != null ? String(s.default) : undefined)}
            value={value === undefined || value === null ? "" : String(value)}
            onChange={(e) => {
              const raw = e.target.value;
              onChange(setAt(spec, path, raw === "" ? undefined : Number(raw)));
            }}
          />
          <Help text={help_} />
          {errorNode}
        </div>
      );

    case "array":
      return (
        <ArrayField
          schema={s}
          path={path}
          spec={spec}
          onChange={onChange}
          errors={errors}
          label={label}
          help={help_}
          hints={hints}
        />
      );

    case "object":
      // object with additionalProperties → map editor
      if (
        (!s.properties || Object.keys(s.properties).length === 0) &&
        s.additionalProperties
      ) {
        return (
          <MapField
            schema={s}
            path={path}
            spec={spec}
            onChange={onChange}
            errors={errors}
            label={label}
            help={help_}
            hints={hints}
          />
        );
      }
      // nested group of properties. Children are ordered by their hint.order
      // (the backend serializes Go maps alphabetically, so insertion order is
      // NOT meaningful) and — on the basic screen only — filtered to basic-tier
      // or required keys (FIX 3).
      {
        const required = new Set(s.required ?? []);
        const children = Object.entries(s.properties ?? {})
          .map(([k, child]) => {
            const hintKey = [...path, k].join(".");
            const hint = hints?.fields[hintKey];
            return { k, child: asSchema(child), hint };
          })
          .filter(({ k, hint }) => {
            if (!basicScreen) return true; // advanced/expert: render everything
            return hint?.tier === "basic" || required.has(k);
          })
          .sort(
            (a, b) =>
              (a.hint?.order ?? 500) - (b.hint?.order ?? 500) ||
              a.k.localeCompare(b.k),
          );
        return (
          <fieldset className="space-y-3 rounded-md border border-border/60 p-3">
            {label && <legend className="px-1 text-sm font-medium">{label}</legend>}
            <Help text={help_} />
            {children.map(({ k, child, hint }) => (
              <Field
                key={k}
                schema={child}
                path={[...path, k]}
                spec={spec}
                onChange={onChange}
                errors={errors}
                label={hint?.label ?? humanize(k)}
                help={hint?.help ?? child.description}
                placeholder={hint?.example}
                hints={hints}
                basicScreen={basicScreen}
              />
            ))}
            {errorNode}
          </fieldset>
        );
      }

    default: {
      // string (with pattern validation) — textarea for long content fields.
      const isMultiline = /content|description|body/i.test(pathToString(path));
      const commonProps = {
        id,
        value: value === undefined || value === null ? "" : String(value),
        placeholder,
        onChange: (
          e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>,
        ) => onChange(setAt(spec, path, e.target.value || undefined)),
      };
      const patternInvalid =
        patternRe && value != null && value !== "" && !patternRe.test(String(value));
      return (
        <div className="space-y-1">
          {label && <Label id={id}>{label}</Label>}
          {isMultiline ? (
            <Textarea {...commonProps} />
          ) : (
            <Input {...commonProps} pattern={s.pattern} />
          )}
          <Help text={help_} />
          {patternInvalid && (
            <p className="text-xs text-destructive">Value must match pattern {s.pattern}</p>
          )}
          {errorNode}
        </div>
      );
    }
  }
}

// Icon-only destructive control. `size="icon"` carries the 40px hit area; the
// visible glyph is smaller, which is why the accessible name is mandatory.
export function RemoveButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <Button
      type="button"
      size="icon"
      variant="ghost"
      aria-label={label}
      title={label}
      className="text-muted-foreground hover:text-destructive"
      onClick={onClick}
    >
      <XIcon />
    </Button>
  );
}

function ArrayField({
  schema,
  path,
  spec,
  onChange,
  errors,
  label,
  help,
  hints,
}: {
  schema: JSONSchema;
  path: PathSeg[];
  spec: unknown;
  onChange: (next: unknown) => void;
  errors: FieldError[];
  label?: string;
  help?: string;
  hints?: UIHints;
}) {
  const item = asSchema(schema.items);
  const arr = (getAt(spec, path) as unknown[] | undefined) ?? [];
  const emptyItem = () => (item.type === "object" ? {} : undefined);
  return (
    <div className="space-y-2">
      {label && <Label>{label}</Label>}
      <Help text={help} />
      <div className="space-y-3">
        {arr.map((_, i) => (
          <div key={i} className="rounded-md border border-border/60 p-3">
            <div className="mb-2 flex items-center justify-between">
              <Badge variant="secondary">#{i + 1}</Badge>
              <RemoveButton
                label={`Remove item ${i + 1}`}
                onClick={() => onChange(deleteAt(spec, [...path, i]))}
              />
            </div>
            <Field
              schema={item}
              path={[...path, i]}
              spec={spec}
              onChange={onChange}
              errors={errors}
              hints={hints}
            />
          </div>
        ))}
      </div>
      <Button
        type="button"
        size="sm"
        variant="outline"
        onClick={() => onChange(setAt(spec, [...path, arr.length], emptyItem()))}
      >
        + Add item
      </Button>
    </div>
  );
}

// Editor for an `additionalProperties` map (e.g. the App XRD's `configs` and
// `labels`). Two things make this more than a list of pairs:
//
//   1. Row order lives in LOCAL state, not in the spec. An unnamed row has no
//      key, and a key-less entry cannot be represented in a JSON object — so
//      deriving the rows from the spec made "+ Add" write nothing and render
//      nothing (the button was inert). Rows are local; the spec is the output.
//   2. The value editor follows `additionalProperties`. When the map's values
//      are objects (`configs` is `{path, content}`), a bare text input would
//      produce a string where the schema demands an object and the claim would
//      be rejected server-side. Object values recurse into <Field/>.
function MapField({
  schema,
  path,
  spec,
  onChange,
  errors,
  label,
  help,
  hints,
}: {
  schema: JSONSchema;
  path: PathSeg[];
  spec: unknown;
  onChange: (next: unknown) => void;
  errors: FieldError[];
  label?: string;
  help?: string;
  hints?: UIHints;
}) {
  const valueSchema = asSchema(schema.additionalProperties);
  const objectValued =
    valueSchema.type === "object" ||
    !!(valueSchema.properties && Object.keys(valueSchema.properties).length > 0);

  const map = (getAt(spec, path) as Record<string, unknown> | undefined) ?? {};
  const specKeys = Object.keys(map);

  // `detached` holds a row's value while the row has no key. A key-less entry
  // cannot exist in a JSON object, so with nowhere to park it, backspacing
  // through a key to fix a typo destroyed whatever was under it.
  type Row = { id: number; key: string; detached?: unknown };
  const [rows, setRows] = useState<Row[]>(() => specKeys.map((key, i) => ({ id: i, key })));
  const nextId = useRef(rows.length);

  // Reconcile with keys that changed in the spec from OUTSIDE this widget: an
  // edit-mode load, or the AI prefill assist replacing the whole map. Rows whose
  // key the spec no longer has are dropped — leaving them mounted let a stale row
  // re-create an entry the prefill had just removed.
  const specKeySignature = specKeys.join(" ");
  useEffect(() => {
    setRows((rs) => {
      const live = new Set(specKeys);
      const kept = rs.filter((r) => r.key === "" || live.has(r.key));
      const known = new Set(kept.map((r) => r.key));
      const missing = specKeys.filter((k) => !known.has(k));
      if (kept.length === rs.length && missing.length === 0) return rs;
      return [...kept, ...missing.map((key) => ({ id: nextId.current++, key }))];
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [specKeySignature]);

  const emptyValue = () => (objectValued ? {} : "");

  // Rewrite the map in ONE setAt, preserving insertion order. Delete-then-set
  // moved a renamed key to the end of the object on every keystroke, reshuffling
  // the live YAML pane and re-firing the reconcile effect per character.
  const writeMap = (next: Record<string, unknown>) =>
    onChange(setAt(spec, path, Object.keys(next).length ? next : undefined));

  const renameKey = (id: number, from: string, to: string) => {
    // Refuse a key another row already holds. Accepting it overwrote that row's
    // value and left two rows aliasing one entry, so removing either wiped both.
    if (to !== "" && rows.some((r) => r.id !== id && r.key === to)) {
      setRows((rs) => rs.map((r) => (r.id === id ? { ...r, key: to } : r)));
      return;
    }

    const held = from ? map[from] : rows.find((r) => r.id === id)?.detached;
    setRows((rs) =>
      rs.map((r) => (r.id === id ? { ...r, key: to, detached: to === "" ? held : undefined } : r)),
    );

    const next: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(map)) {
      if (k === from) {
        if (to !== "") next[to] = held ?? emptyValue(); // re-inserted in place
        continue;
      }
      next[k] = v;
    }
    if (to !== "" && !(from && from in map)) next[to] = held ?? emptyValue();
    writeMap(next);
  };

  const removeRow = (id: number, key: string) => {
    setRows((rs) => rs.filter((r) => r.id !== id));
    if (!key) return;
    const next = { ...map };
    delete next[key];
    writeMap(next);
  };

  return (
    <div className="space-y-2">
      {label && <Label>{label}</Label>}
      <Help text={help} />
      <div className="space-y-3">
        {rows.map(({ id, key }) => (
          <div
            key={id}
            className={objectValued ? "space-y-2 rounded-md border border-border/60 p-3" : undefined}
          >
            <div className="flex gap-2">
              <Input
                aria-label="Key"
                placeholder="key"
                value={key}
                onChange={(e) => renameKey(id, key, e.target.value)}
              />
              {!objectValued && (
                <Input
                  aria-label="Value"
                  placeholder="value"
                  value={key ? String(map[key] ?? "") : ""}
                  disabled={!key}
                  onChange={(e) =>
                    onChange(setAt(spec, [...path, key], e.target.value || undefined))
                  }
                />
              )}
              <RemoveButton label={`Remove ${key || "entry"}`} onClick={() => removeRow(id, key)} />
            </div>
            {objectValued &&
              (key ? (
                <Field
                  schema={valueSchema}
                  path={[...path, key]}
                  spec={spec}
                  onChange={onChange}
                  errors={errors}
                  hints={hints}
                />
              ) : (
                <p className="text-xs text-muted-foreground">Name the entry to configure it.</p>
              ))}
          </div>
        ))}
      </div>
      <Button
        type="button"
        size="sm"
        variant="outline"
        onClick={() => setRows((rs) => [...rs, { id: nextId.current++, key: "" }])}
      >
        + Add entry
      </Button>
    </div>
  );
}
