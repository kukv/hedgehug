import { describe, expect, it } from "vitest";
import { NotFound } from "../../model/errors";
import { get } from "../../reactive/store";
import { fakeLinks } from "./fixtures.test-util";
import { createRepoFailures, scanFailedText } from "./repoFailures";

describe("scanFailedText", () => {
  it("puts the reason after the fixed sentence", () => {
    expect(scanFailedText("GitHub に接続できません")).toBe(
      "PR を探せませんでした: GitHub に接続できません",
    );
  });
});

describe("createRepoFailures", () => {
  it("reads the failures of the open project and finds a text ignoring case", async () => {
    const links = fakeLinks();
    links.failures = [{ repo: "example-org/web", text: "利用上限に達しました" }];
    const f = createRepoFailures({ links: links.port });
    await f.reload(); // 開いていなければ何もしない
    expect(links.calls).toEqual([]);
    await f.load("S1", "P1");
    expect(links.calls).toEqual(["failures S1 P1"]);
    expect(f.failureOf("Example-Org/web")).toBe("利用上限に達しました");
    expect(f.failureOf("example-org/api")).toBe("");
  });

  it("reads again on reload and drops a failure that went away", async () => {
    const links = fakeLinks();
    links.failures = [{ repo: "org/web", text: "x" }];
    const f = createRepoFailures({ links: links.port });
    await f.load("S1", "P1");
    links.failures = [];
    await f.reload();
    expect(f.failureOf("org/web")).toBe("");
    expect(get(f.state).loaded).toBe(true);
  });

  it("empties when the project is gone and when it is reset", async () => {
    const links = fakeLinks();
    links.failures = [{ repo: "org/web", text: "x" }];
    const f = createRepoFailures({ links: links.port });
    await f.load("S1", "P1");
    links.fail = new NotFound();
    await f.reload();
    expect(f.failureOf("org/web")).toBe("");
    links.fail = null;
    await f.reload();
    expect(f.failureOf("org/web")).toBe("x");
    f.reset();
    expect(f.failureOf("org/web")).toBe("");
  });
});
