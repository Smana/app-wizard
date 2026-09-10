import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";

vi.mock("../api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api/client")>();
  return {
    ...actual,
    listApps: vi.fn().mockResolvedValue([
      { stack: "dev", name: "cinema", namespace: "dev-apps", image: "cinema:1.0", type: "web" },
      { stack: "prod", name: "reaper", namespace: "prod-apps", image: "reaper:2.1", type: "cron" },
    ]),
    openPR: vi.fn(),
  };
});

import { AppList } from "./AppList";

const headlamp = { label: "Headlamp (aws-0)", url: "https://h.example/c/main/apps/{namespace}/{name}" };
const runbook = { label: "Runbook", url: "https://wiki.example/{stack}/{name}" };

async function renderCards(links?: { label: string; url: string }[]) {
  render(<AppList onEdit={vi.fn()} links={links} />);
  await waitFor(() => {
    expect(screen.getAllByTestId("app-card")).toHaveLength(2);
  });
}

describe("AppList", () => {
  it("renders a card per app with edit and decommission", async () => {
    await renderCards();
    expect(screen.getByText("cinema")).toBeTruthy();
    expect(screen.getByText("reaper")).toBeTruthy();
    expect(screen.getAllByRole("button", { name: /Edit/i })).toHaveLength(2);
    expect(screen.getAllByRole("button", { name: /Decommission/i })).toHaveLength(2);
  });

  it("has no open action when no links are configured", async () => {
    await renderCards([]);
    expect(screen.queryByTestId("app-open")).toBeNull();
    expect(screen.queryByTestId("app-open-menu")).toBeNull();
  });

  it("with one link, the open action is a new-tab anchor expanded for the app", async () => {
    await renderCards([headlamp]);
    const opens = screen.getAllByTestId("app-open") as HTMLAnchorElement[];
    expect(opens).toHaveLength(2);
    expect(opens[0].getAttribute("href")).toBe("https://h.example/c/main/apps/dev-apps/cinema");
    expect(opens[0].getAttribute("target")).toBe("_blank");
    expect(opens[0].getAttribute("rel")).toContain("noopener");
    expect(opens[0].textContent).toContain("Headlamp (aws-0)");
  });

  it("with several links, the open action is a menu listing each label", async () => {
    await renderCards([headlamp, runbook]);
    const menus = screen.getAllByTestId("app-open-menu");
    expect(menus).toHaveLength(2);
    fireEvent.click(menus[0].querySelector("summary")!);
    const items = menus[0].querySelectorAll("a");
    expect(items).toHaveLength(2);
    expect(items[0].getAttribute("href")).toBe("https://h.example/c/main/apps/dev-apps/cinema");
    expect(items[1].getAttribute("href")).toBe("https://wiki.example/dev/cinema");
    expect(screen.queryByTestId("app-open")).toBeNull();
  });

  it("keys duplicate-label menu items uniquely, with no React duplicate-key warning", async () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const runbookA = { label: "Runbook", url: "https://wiki.example/a/{name}" };
    const runbookB = { label: "Runbook", url: "https://wiki.example/b/{name}" };
    await renderCards([runbookA, runbookB]);
    const menus = screen.getAllByTestId("app-open-menu");
    fireEvent.click(menus[0].querySelector("summary")!);
    const items = menus[0].querySelectorAll("a");
    expect(items).toHaveLength(2);
    expect(items[0].getAttribute("href")).toBe("https://wiki.example/a/cinema");
    expect(items[1].getAttribute("href")).toBe("https://wiki.example/b/cinema");
    const keyWarnings = errorSpy.mock.calls.filter((args) => String(args[0]).includes("same key"));
    expect(keyWarnings).toHaveLength(0);
    errorSpy.mockRestore();
  });
});
