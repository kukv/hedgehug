import { describe, expect, it } from "vitest";
import {
  checksLabel,
  eventLabel,
  fetchedText,
  fetchLine,
  fetchTimes,
  fullRepo,
  type GitHubRepo,
  installUrl,
  issueState,
  type Link,
  linkArg,
  linkUrl,
  type MissingApp,
  missingByOwner,
  parseApplied,
  parseRejected,
  prChip,
  prState,
  type Rule,
  rateLine,
  repoLabel,
  repoOf,
  reviewLabel,
  ruleHint,
  slowedText,
  syncedFields,
  viaLabel,
} from "./github";

const pr = (n: number, over: Partial<Link> = {}): Link => ({
  type: "github_pr",
  ref: `github.com/org-a/web/pull/${n}`,
  label: `org-a/web#${n}`,
  sync: "mirror",
  state: "open",
  review: "none",
  ...over,
});
const issue = (over: Partial<Link> = {}): Link => ({
  type: "github_issue",
  ref: "github.com/org-a/web/issues/45",
  label: "org-a/web#45",
  sync: "push",
  ...over,
});

describe("prState", () => {
  it.each<[Partial<Link>, string, string]>([
    [{ state: "merged", checks: "failure", review: "changes_requested" }, "merged", "merged"],
    [{ state: "draft", checks: "failure", review: "approved" }, "draft", "draft"],
    [{ checks: "failure", review: "changes_requested" }, "CI 失敗", "fail"],
    [{ review: "changes_requested" }, "修正依頼", "changes"],
    [{ review: "approved", checks: "success" }, "approved", "open"],
    [{ review: "pending", checks: "pending" }, "open", "open"],
    [{ state: "closed", checks: "failure" }, "closed", "closed"],
    [{ state: "" }, "", "unread"],
  ])("%j", (over, text, tone) => {
    expect(prState(pr(1, over))).toEqual({ text, tone });
  });
});

describe("prChip", () => {
  it("is null with no pull request", () => {
    expect(prChip(undefined)).toBeNull();
    expect(prChip([issue()])).toBeNull();
  });

  it("shows the name without the owner, and the page of the pull request", () => {
    expect(prChip([pr(88)])).toEqual({
      text: "open",
      tone: "open",
      label: "web#88",
      href: "https://github.com/org-a/web/pull/88",
      more: 0,
    });
  });

  it("shows the ones not merged first and counts the others", () => {
    const chip = prChip([pr(1, { state: "merged" }), pr(2, { state: "closed" }), pr(3)]);
    expect(chip?.label).toBe("web#3");
    expect(chip?.more).toBe(2);
  });

  it("keeps the order of the links within a rank", () => {
    expect(prChip([pr(2), pr(1, { state: "draft" })])?.label).toBe("web#2");
    expect(prChip([pr(1, { state: "merged" }), pr(2, { state: "merged" })])?.label).toBe("web#1");
  });
});

describe("the addresses of a link", () => {
  it("reads the repository from the ref", () => {
    expect(repoOf("github.com/org-a/web/pull/88")).toBe("github.com/org-a/web");
    expect(repoLabel("github.com/org-a/web")).toBe("org-a/web");
    expect(repoLabel("ghe.example.com/org-a/web")).toBe("ghe.example.com/org-a/web");
    expect(linkUrl(pr(88))).toBe("https://github.com/org-a/web/pull/88");
  });

  it("writes a repository with its host", () => {
    expect(fullRepo(" Org-A/Web ")).toBe("github.com/org-a/web");
    expect(fullRepo("ghe.example.com/org-a/web")).toBe("ghe.example.com/org-a/web");
  });

  it("names the host of a link that is not on github.com", () => {
    expect(linkArg(pr(88))).toBe("org-a/web#88");
    expect(linkArg(pr(1, { ref: "ghe.example.com/org-a/web/pull/1" }))).toBe(
      "ghe.example.com/org-a/web#1",
    );
  });
});

describe("the words", () => {
  it("shows no review for none and for a draft", () => {
    expect(reviewLabel(pr(1, { review: "none" }))).toBe("—");
    expect(reviewLabel(pr(1, { state: "draft", review: "approved" }))).toBe("—");
    expect(reviewLabel(pr(1, { review: "pending" }))).toBe("レビュー待ち");
    expect(reviewLabel(pr(1, { review: "changes_requested" }))).toBe("修正依頼");
  });

  it("names the CI", () => {
    expect(checksLabel(pr(1, { checks: "pending" }))).toBe("CI 実行中");
    expect(checksLabel(pr(1))).toBe("");
  });

  it("names where a link came from", () => {
    expect(viaLabel(pr(1, { via: "branch" }))).toBe("ブランチ名から自動リンク");
    expect(viaLabel(pr(1))).toBe("");
  });

  it("tells a closed issue by its reason", () => {
    expect(issueState(issue()).tone).toBe("unread");
    expect(issueState(issue({ state: "open" })).text).toBe("open");
    expect(issueState(issue({ state: "closed" })).text).toBe("closed");
    expect(issueState(issue({ state: "closed", state_reason: "not_planned" })).text).toBe(
      "not planned",
    );
  });

  it("takes the fields of the link, else the ones of the mode", () => {
    const fields = { default_sync: "link", push: ["title", "body"], mirror: ["status"], props: {} };
    expect(syncedFields(issue(), fields)).toBe("タイトル・本文");
    expect(syncedFields(issue({ sync: "mirror" }), fields)).toBe("状態");
    expect(syncedFields(issue({ fields: ["milestone"] }), fields)).toBe("マイルストーン");
    expect(syncedFields(issue({ sync: "link" }), fields)).toBe("");
    expect(syncedFields(issue(), undefined)).toBe("");
  });

  it("names an event, and shows one it does not know as it is", () => {
    expect(eventLabel("pr.opened")).toBe("PR が開かれた");
    expect(eventLabel("x.y")).toBe("x.y");
  });
});

describe("the trailers", () => {
  it("reads a rule that moved a task", () => {
    expect(parseApplied("pr-opened pr.opened")).toEqual({ rule: "pr-opened", event: "pr.opened" });
    expect(parseApplied("nothing")).toBeNull();
  });

  it("reads a rule that was refused, with the spaces of its reason", () => {
    expect(parseRejected("01J pr-merged points: is required")).toEqual({
      task: "01J",
      rule: "pr-merged",
      reason: "points: is required",
    });
    expect(parseRejected("01J")).toBeNull();
    expect(parseRejected("01J r a\\nb\\\\c")?.reason).toBe("a\nb\\c");
  });
});

describe("ruleHint", () => {
  const rule = (over: Partial<Rule>): Rule =>
    ({ id: "r", on: "pr.opened", enabled: true, disabled_in: [], ...over }) as Rule;

  it("tells the two moves a board shows", () => {
    expect(ruleHint(rule({}))).toBe("PR を開くと自動でここへ");
    expect(ruleHint(rule({ on: "pr.merged", when: "all_linked_prs_merged" }))).toBe(
      "全 PR のマージで自動でここへ",
    );
    expect(ruleHint(rule({ on: "pr.merged" }))).toBe("PR のマージで自動でここへ");
    expect(ruleHint(rule({ on: "issue.closed" }))).toBeNull();
  });
});

describe("the times of the reads", () => {
  const now = new Date("2026-11-05T12:00:00");
  const at = (min: number) => new Date(now.getTime() + min * 60_000).toISOString();
  const repo = (name: string, over: Partial<GitHubRepo>): GitHubRepo =>
    ({
      repo: name,
      store: "s",
      app_missing: false,
      limit: 0,
      remaining: 0,
      warnings: [],
      ...over,
    }) as GitHubRepo;

  it("takes the latest read and the first to come of the repositories named", () => {
    const all = [
      repo("a", { last_fetch: at(-9), next_fetch: at(1), slowed_until: at(5) }),
      repo("b", { last_fetch: at(-3), next_fetch: at(2), slowed_until: at(8) }),
      repo("c", { last_fetch: at(-1), next_fetch: at(0) }),
    ];
    expect(fetchTimes(all, new Set(["a", "b"]))).toEqual({
      last: at(-3),
      next: at(1),
      slowedUntil: at(8),
    });
    expect(fetchTimes(all, new Set())).toEqual({});
  });

  it("says when it was read", () => {
    expect(fetchedText(undefined, now)).toBe("");
    expect(fetchedText(at(0), now)).toBe("たった今取得");
    expect(fetchedText(at(-3), now)).toBe("3 分前に取得");
  });

  it("says the next read only when it is still to come", () => {
    expect(fetchLine({}, now)).toBe("GitHub からまだ取得していません");
    expect(fetchLine({ last: at(-3), next: at(2) }, now)).toBe(
      "GitHub から取得: 3 分前 · 次は 2 分後",
    );
    expect(fetchLine({ last: at(-3), next: at(-1) }, now)).toBe("GitHub から取得: 3 分前");
  });

  it("shows the band only until the recovery", () => {
    expect(slowedText(undefined, now)).toBeNull();
    expect(slowedText(at(-1), now)).toBeNull();
    expect(slowedText(at(30), now)).toBe("取得の間隔を 15 分に延ばしています · 12:30 に回復");
  });
});

describe("the reads as the settings show them", () => {
  const repo = (over: Partial<GitHubRepo>): GitHubRepo => ({
    store: "shared",
    repo: "github.com/org-b/ops",
    warnings: [],
    limit: 0,
    remaining: 0,
    app_missing: false,
    ...over,
  });

  it("gives the rate limit of the repository with the least left", () => {
    expect(rateLine([repo({})])).toBeNull();
    expect(
      rateLine([repo({ limit: 5000, remaining: 4900 }), repo({ limit: 5000, remaining: 4812 })]),
    ).toBe("レート制限 4,812 / 5,000");
  });

  it("groups what an account cannot read by owner", () => {
    const all = [
      repo({ account: "work", app_missing: true }),
      repo({ store: "me", account: "work", app_missing: true }),
      repo({ account: "work", app_missing: true, repo: "github.com/org-b/api" }),
      repo({ account: "me", app_missing: true }),
      repo({ account: "work", repo: "github.com/org-a/web" }),
    ];
    const [m, ...rest] = missingByOwner(all, "work");
    expect(rest).toEqual([]);
    expect(m).toEqual({ host: "github.com", owner: "org-b", repos: ["org-b/ops", "org-b/api"] });
    expect(installUrl(m as MissingApp)).toBe(
      "https://github.com/organizations/org-b/settings/installations",
    );
  });
});
