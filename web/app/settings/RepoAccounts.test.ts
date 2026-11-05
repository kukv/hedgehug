import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import { ok } from "../detail/testkit";
import { emitter, renderWithCore } from "../test-utils";
import RepoAccounts from "./RepoAccounts.svelte";

const rules = [
  { match: "github.com/org-a/*", account: "work" },
  { match: "*", account: "personal" },
];

function setup() {
  const e = emitter();
  const api = {
    GET: vi.fn(() => ok({ rules })),
    PUT: vi.fn((_: string, o: { body: { rules: unknown[] } }) => ok(o.body)),
  };
  renderWithCore(
    RepoAccounts,
    {},
    {
      api,
      accounts: {
        accounts: () => [{ id: "github.com/me", name: "personal" }],
        subscribe: e.subscribe,
      },
    },
  );
  return { api };
}

const put = (next: unknown[]) => ({ body: { rules: next } });

describe("RepoAccounts", () => {
  it("lists the rules in order", async () => {
    setup();
    await screen.findByText("github.com/org-a/*");
    expect(
      screen.getAllByRole("listitem").map((li) => li.querySelector("code")?.textContent),
    ).toEqual(["github.com/org-a/*", "*"]);
  });

  it("moves a rule with the buttons", async () => {
    const { api } = setup();
    await fireEvent.click(await screen.findByRole("button", { name: "上へ: *" }));
    await waitFor(() =>
      expect(api.PUT).toHaveBeenCalledWith("/api/repo-accounts", put([rules[1], rules[0]])),
    );
    await waitFor(() => expect(screen.getByRole("button", { name: "上へ: *" })).toBeDisabled());
  });

  it("removes a rule", async () => {
    const { api } = setup();
    await fireEvent.click(await screen.findByRole("button", { name: "削除: *" }));
    await waitFor(() =>
      expect(api.PUT).toHaveBeenCalledWith("/api/repo-accounts", put([rules[0]])),
    );
  });

  it("adds a rule at the end", async () => {
    const { api } = setup();
    await screen.findByText("*");
    await fireEvent.click(screen.getByRole("button", { name: "ルールを追加" }));
    await fireEvent.input(screen.getByLabelText("リポジトリ"), {
      target: { value: "github.com/org-b/*" },
    });
    await fireEvent.change(screen.getByLabelText("アカウント"), {
      target: { value: "personal" },
    });
    await fireEvent.click(screen.getByRole("button", { name: "追加" }));
    await waitFor(() =>
      expect(api.PUT).toHaveBeenCalledWith(
        "/api/repo-accounts",
        put([...rules, { match: "github.com/org-b/*", account: "personal" }]),
      ),
    );
    await screen.findByRole("button", { name: "ルールを追加" });
  });
});
