import { describe, expect, it } from "vitest";
import { NotFound } from "../../model/errors";
import { get } from "../../reactive/store";
import { fakeLinks, row } from "./fixtures.test-util";
import { createProjectLinks } from "./projectLinks";

describe("createProjectLinks", () => {
  it("reads the rows of the open project and reads them again", async () => {
    const links = fakeLinks();
    const rows = [{ ...row(), item: "I1", display: "HH-2" }];
    links.port.listProject = async (s, p) => {
      links.calls.push(`project ${s} ${p}`);
      return rows;
    };
    const p = createProjectLinks({ links: links.port });
    await p.reload(); // 開いていなければ何もしない
    await p.load("S1", "P1");
    await p.reload();
    expect(links.calls).toEqual(["project S1 P1", "project S1 P1"]);
    expect(get(p.state)).toEqual({ loaded: true, rows });
  });

  it("empties when the project is gone", async () => {
    const links = fakeLinks();
    links.port.listProject = async () => {
      throw new NotFound();
    };
    const p = createProjectLinks({ links: links.port });
    await p.load("S1", "P1");
    expect(get(p.state)).toEqual({ loaded: true, rows: [] });
  });
});
