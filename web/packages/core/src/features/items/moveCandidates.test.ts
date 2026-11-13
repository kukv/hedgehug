import { describe, expect, it } from "vitest";
import type { MoveCandidate } from "../../model/itemAction";
import { filterMoveCandidates } from "./moveCandidates";

const c = (display: string, title: string, projectTitle = "Web"): MoveCandidate => ({
  kind: "item",
  id: `id-${display}`,
  display,
  title,
  projectId: "P1",
  projectTitle,
});
const all = [c("HH-3", "認証の改善"), c("HH-30", "Search 画面"), c("HH-4", "決済 and HH-3")];

describe("filterMoveCandidates", () => {
  it("returns everything for an empty or blank query", () => {
    expect(filterMoveCandidates(all, "")).toBe(all);
    expect(filterMoveCandidates(all, "  ")).toBe(all);
  });

  it("matches the start of the ID, ignoring case", () => {
    expect(filterMoveCandidates(all, "hh-30").map((x) => x.display)).toEqual(["HH-30"]);
    // 「hh-3」は HH-3・HH-30 の ID の前方一致と、タイトルに hh-3 を含む HH-4 に合う。
    expect(filterMoveCandidates(all, "hh-3").map((x) => x.display)).toEqual([
      "HH-3",
      "HH-30",
      "HH-4",
    ]);
    // ID の途中からは合わない(「H-3」は ID には合わず、タイトルの部分一致で HH-4 だけが合う)。
    expect(filterMoveCandidates(all, "H-3").map((x) => x.display)).toEqual(["HH-4"]);
  });

  it("matches a part of the title, ignoring case", () => {
    expect(filterMoveCandidates(all, "search").map((x) => x.display)).toEqual(["HH-30"]);
    expect(filterMoveCandidates(all, "の改")).toEqual([all[0]]);
  });

  it("trims the query and returns nothing when nothing matches", () => {
    expect(filterMoveCandidates(all, " 決済 ").map((x) => x.display)).toEqual(["HH-4"]);
    expect(filterMoveCandidates(all, "zzz")).toEqual([]);
  });

  it("matches project candidates by their display ID too", () => {
    const p: MoveCandidate = {
      kind: "project",
      id: "P2",
      display: "HH-P2",
      title: "モバイル",
      projectId: "P2",
      projectTitle: "",
    };
    expect(filterMoveCandidates([p, ...all], "hh-p")).toEqual([p]);
  });
});
