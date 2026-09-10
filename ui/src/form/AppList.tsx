// Day-2 inventory view (Phase 2). Lists apps declared across all stacks as
// cards and offers Edit / Decommission, plus the operator-configured open
// links, per card.
import { useCallback, useEffect, useState } from "react";
import type { AppSummary, Link, PRResponse } from "../api/types";
import * as api from "../api/client";
import { Alert, AlertDescription, AlertTitle } from "../components/ui/alert";
import { Badge } from "../components/ui/badge";
import { Button, buttonVariants } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { errorMessage } from "../lib/utils";
import { expandLink } from "./links";

interface Props {
  // Called when a card's "Edit" action is triggered — parent fetches the detail
  // and swaps in the wizard.
  onEdit: (app: AppSummary) => void;
  // Operator-configured external links (from /api/branding). One link renders
  // as a direct open action; several render as a menu. None: cards stay inert.
  links?: Link[];
}

// The open action of one card. `<details>` gives a keyboard-accessible menu
// with no state to manage; the anchors open in a new tab and drop the opener.
function OpenAction({ app, links }: { app: AppSummary; links: Link[] }) {
  if (links.length === 0) return null;
  if (links.length === 1) {
    const l = links[0];
    return (
      <a
        data-testid="app-open"
        className={buttonVariants({ variant: "default", size: "sm" })}
        href={expandLink(l.url, app)}
        target="_blank"
        rel="noopener noreferrer"
      >
        Open in {l.label}
      </a>
    );
  }
  return (
    <details data-testid="app-open-menu" className="relative">
      <summary className={buttonVariants({ variant: "default", size: "sm" }) + " cursor-pointer list-none"}>
        Open ▾
      </summary>
      <ul className="absolute right-0 z-10 mt-1 min-w-48 rounded-md border border-border bg-card p-1 shadow-border">
        {links.map((l) => (
          <li key={`${l.label}-${l.url}`}>
            <a
              className="block rounded px-3 py-2 text-sm hover:bg-muted"
              href={expandLink(l.url, app)}
              target="_blank"
              rel="noopener noreferrer"
            >
              {l.label}
            </a>
          </li>
        ))}
      </ul>
    </details>
  );
}

type LoadState = "loading" | "loaded" | "error";

export function AppList({ onEdit, links = [] }: Props) {
  const [apps, setApps] = useState<AppSummary[]>([]);
  const [state, setState] = useState<LoadState>("loading");
  const [error, setError] = useState<string | null>(null);

  // Per-row decommission progress + resulting PR / error.
  const [decommissioning, setDecommissioning] = useState<string | null>(null);
  const [removalPr, setRemovalPr] = useState<PRResponse | null>(null);
  const [decommissionError, setDecommissionError] = useState<string | null>(null);

  const rowKey = (a: AppSummary) => `${a.stack}/${a.name}`;

  const load = useCallback(() => {
    setState("loading");
    setError(null);
    api
      .listApps()
      .then((res) => {
        setApps(res);
        setState("loaded");
      })
      .catch((e) => {
        setError(errorMessage(e));
        setState("error");
      });
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function onDecommission(app: AppSummary) {
    const ok = window.confirm(
      `Decommission "${app.name}" (stack ${app.stack})?\n\n` +
        "This opens a pull request that removes the app's manifest from Git.",
    );
    if (!ok) return;

    setDecommissioning(rowKey(app));
    setDecommissionError(null);
    setRemovalPr(null);
    try {
      const res = await api.openPR({
        stack: app.stack,
        appName: app.name,
        mode: "delete",
        spec: {},
        description: `Decommission ${app.name}`,
      });
      setRemovalPr(res);
    } catch (e) {
      setDecommissionError(errorMessage(e));
    } finally {
      setDecommissioning(null);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">My apps</h2>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={load}
          disabled={state === "loading"}
        >
          {state === "loading" ? "Refreshing…" : "Refresh"}
        </Button>
      </div>

      {removalPr && (
        <Alert variant="success">
          <AlertTitle>Decommission pull request opened</AlertTitle>
          <AlertDescription>
            <a
              className="text-primary underline"
              href={removalPr.url}
              target="_blank"
              rel="noreferrer"
            >
              {removalPr.url}
            </a>{" "}
            (#{removalPr.number}, branch <code>{removalPr.branch}</code>)
          </AlertDescription>
        </Alert>
      )}

      {decommissionError && (
        <Alert variant="destructive">
          <AlertTitle>Could not open decommission PR</AlertTitle>
          <AlertDescription>{decommissionError}</AlertDescription>
        </Alert>
      )}

      {state === "loading" && (
        <p className="text-sm text-muted-foreground">Loading apps…</p>
      )}

      {state === "error" && (
        <Alert variant="destructive">
          <AlertTitle>Could not load apps</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {state === "loaded" && apps.length === 0 && (
        <Card>
          <CardContent className="py-8 text-center text-sm text-muted-foreground">
            No apps yet — create one.
          </CardContent>
        </Card>
      )}

      {state === "loaded" && apps.length > 0 && (
        <div className="space-y-3">
          <p className="text-sm text-muted-foreground">
            {apps.length} app{apps.length === 1 ? "" : "s"}
          </p>
          <ul className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3" aria-label="Apps">
            {apps.map((app) => (
              <li key={rowKey(app)}>
                <Card data-testid="app-card" className="flex h-full flex-col">
                  <CardHeader className="flex-row items-start justify-between gap-2 space-y-0">
                    <div className="min-w-0">
                      <CardTitle className="truncate">{app.name}</CardTitle>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {app.stack} · {app.namespace}
                      </p>
                    </div>
                    <Badge variant="secondary">{app.type || "web"}</Badge>
                  </CardHeader>
                  <CardContent className="flex flex-1 flex-col justify-between gap-4">
                    <p className="truncate font-mono text-xs text-muted-foreground" title={app.image}>
                      {app.image}
                    </p>
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <OpenAction app={app} links={links} />
                      <div className="ml-auto flex items-center gap-2">
                        <Button type="button" variant="outline" size="sm" onClick={() => onEdit(app)}>
                          Edit
                        </Button>
                        <Button
                          type="button"
                          variant="destructive"
                          size="sm"
                          disabled={decommissioning === rowKey(app)}
                          onClick={() => onDecommission(app)}
                        >
                          {decommissioning === rowKey(app) ? "Decommissioning…" : "Decommission"}
                        </Button>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
