// Expands an operator-configured link template for one app. The backend has
// already validated that only these three placeholders occur, so anything
// else is left verbatim rather than guessed at.
import type { AppSummary } from "../api/types";

export type LinkTarget = Pick<AppSummary, "namespace" | "name" | "stack">;

export function expandLink(template: string, app: LinkTarget): string {
  const values: Record<string, string> = {
    namespace: app.namespace,
    name: app.name,
    stack: app.stack,
  };
  return template.replace(/\{(namespace|name|stack)\}/g, (_m, key: string) =>
    encodeURIComponent(values[key]),
  );
}
