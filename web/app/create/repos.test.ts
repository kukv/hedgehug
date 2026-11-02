import { describe, expect, it } from "vitest";
import { repoIdOf } from "./repos";

describe("repoIdOf", () => {
  it("reads https, ssh, git and scp-like URLs the way internal/context.RepoID does", () => {
    expect(repoIdOf("https://github.com/Org-A/Web.git")).toBe("github.com/org-a/web");
    expect(repoIdOf("git@github.com:Org-A/web.git")).toBe("github.com/org-a/web");
    expect(repoIdOf("ssh://git@github.com:22/org-a/web")).toBe("github.com/org-a/web");
    expect(repoIdOf("git://GitHub.com/org-a/web/")).toBe("github.com/org-a/web");
    expect(repoIdOf("http://github.com/org-a/web")).toBe("github.com/org-a/web");
  });

  it("accepts an ID that is already in the form host/owner/repo", () => {
    expect(repoIdOf("github.com/org-a/web")).toBe("github.com/org-a/web");
    expect(repoIdOf("GitHub.com/Org-A/Web.git")).toBe("github.com/org-a/web");
  });

  it("refuses what is not a remote with an owner and a repo", () => {
    expect(repoIdOf("")).toBeNull();
    expect(repoIdOf("/home/me/repo")).toBeNull();
    expect(repoIdOf("file:///x/y")).toBeNull();
    expect(repoIdOf("https://github.com/org-a")).toBeNull();
    expect(repoIdOf("https://github.com/a/b/c")).toBeNull();
    expect(repoIdOf("web")).toBeNull();
  });

  it("ignores surrounding white space", () => {
    expect(repoIdOf("  https://github.com/org-a/web  ")).toBe("github.com/org-a/web");
  });
});
