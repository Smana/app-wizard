// Regression tests for the additionalProperties map editor (Field.tsx →
// MapField). Both cases below were broken: "+ Add entry" wrote nothing because
// rows were derived from the spec (an unnamed row has no key, so it could not
// exist), and object-valued maps like the App XRD's `configs` rendered a single
// text input where the schema demands an object.
import { describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { Field } from "./Field";
import type { JSONSchema } from "./jsonSchema";

// Drives Field the way WizardForm does: one spec in state, replaced wholesale.
function Harness({ schema, initial = {} }: { schema: JSONSchema; initial?: unknown }) {
  const [spec, setSpec] = useState<unknown>(initial);
  return (
    <>
      <Field schema={schema} path={["configs"]} spec={spec} onChange={setSpec} errors={[]} label="Configs" />
      <pre data-testid="spec">{JSON.stringify(spec)}</pre>
    </>
  );
}

const stringValued: JSONSchema = {
  type: "object",
  additionalProperties: { type: "string" },
};

const objectValued: JSONSchema = {
  type: "object",
  additionalProperties: {
    type: "object",
    required: ["path", "content"],
    properties: {
      path: { type: "string" },
      content: { type: "string" },
    },
  },
};

describe("MapField", () => {
  it("adds a usable row when '+ Add entry' is clicked", () => {
    render(<Harness schema={stringValued} />);
    expect(screen.queryAllByLabelText("Key")).toHaveLength(0);

    fireEvent.click(screen.getByText("+ Add entry"));

    expect(screen.getAllByLabelText("Key")).toHaveLength(1);
  });

  it("writes key and value into the spec", () => {
    render(<Harness schema={stringValued} />);
    fireEvent.click(screen.getByText("+ Add entry"));
    fireEvent.change(screen.getByLabelText("Key"), { target: { value: "tier" } });
    fireEvent.change(screen.getByLabelText("Value"), { target: { value: "gold" } });

    expect(JSON.parse(screen.getByTestId("spec").textContent!)).toEqual({
      configs: { tier: "gold" },
    });
  });

  it("keeps the typed value when the key is renamed", () => {
    render(<Harness schema={stringValued} initial={{ configs: { old: "keep-me" } }} />);
    fireEvent.change(screen.getByLabelText("Key"), { target: { value: "new" } });

    expect(JSON.parse(screen.getByTestId("spec").textContent!)).toEqual({
      configs: { new: "keep-me" },
    });
  });

  it("removes a row from the spec", () => {
    render(<Harness schema={stringValued} initial={{ configs: { gone: "x" } }} />);
    fireEvent.click(screen.getByLabelText("Remove gone"));

    expect(JSON.parse(screen.getByTestId("spec").textContent!)).toEqual({ configs: {} });
  });

  it("renders the object sub-fields for an object-valued map, not a text input", () => {
    render(<Harness schema={objectValued} />);
    fireEvent.click(screen.getByText("+ Add entry"));
    // Nothing to configure until the entry is named.
    expect(screen.getByText("Name the entry to configure it.")).toBeTruthy();

    fireEvent.change(screen.getByLabelText("Key"), { target: { value: "nginx.conf" } });

    // The value editor is the recursed object, so `path` and `content` appear as
    // their own inputs — a single "value" text box would be the old bug.
    expect(screen.queryByLabelText("Value")).toBeNull();
    expect(screen.getByLabelText("Path")).toBeTruthy();
    expect(screen.getByLabelText("Content")).toBeTruthy();
  });

  it("writes object values at the right path", () => {
    render(<Harness schema={objectValued} />);
    fireEvent.click(screen.getByText("+ Add entry"));
    fireEvent.change(screen.getByLabelText("Key"), { target: { value: "nginx.conf" } });
    fireEvent.change(screen.getByLabelText("Path"), { target: { value: "/etc/nginx.conf" } });

    expect(JSON.parse(screen.getByTestId("spec").textContent!)).toEqual({
      configs: { "nginx.conf": { path: "/etc/nginx.conf" } },
    });
  });
});
