import { describe, expect, it } from "vitest";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createContextLoader } from "./createDialogLoad";
import { fakeWorkspaces } from "./settings.test-util";

describe("createContextLoader", () => {
  it("reads again after a failure instead of remembering it", async () => {
    const workspaces = fakeWorkspaces();
    let fail = true;
    const get = workspaces.get;
    workspaces.get = async (s, w) => {
      if (fail) throw new Error("down");
      return get(s, w);
    };
    const load = createContextLoader(workspaces, fakeCatalogPort());
    await expect(load("S1", "S1-W1")).rejects.toThrow("down");
    fail = false;
    await expect(load("S1", "S1-W1")).resolves.toMatchObject({ people: { me: "taro" } });
  });
});
