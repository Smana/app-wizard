import { describe, expect, it } from "vitest";
import { expandLink } from "./links";

const app = { namespace: "demo", name: "podinfo", stack: "demo" };

describe("expandLink", () => {
  it("substitutes every allowed placeholder", () => {
    expect(expandLink("https://h.example/c/main/apps/{namespace}/{name}?s={stack}", app)).toBe(
      "https://h.example/c/main/apps/demo/podinfo?s=demo",
    );
  });

  it("substitutes a placeholder used twice", () => {
    expect(expandLink("https://x/{name}/{name}", app)).toBe("https://x/podinfo/podinfo");
  });

  it("URL-encodes values", () => {
    expect(expandLink("https://x/{name}", { ...app, name: "a b/c" })).toBe("https://x/a%20b%2Fc");
  });

  it("leaves a template with no placeholders untouched", () => {
    expect(expandLink("https://x/static", app)).toBe("https://x/static");
  });
});
