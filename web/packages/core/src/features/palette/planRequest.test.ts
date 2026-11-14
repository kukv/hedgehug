import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createPlanRequest } from "./planRequest";

describe("createPlanRequest", () => {
  it("is null at first", () => {
    expect(get(createPlanRequest().request)).toBeNull();
  });

  it("keeps the request when another workspace takes", () => {
    const p = createPlanRequest();
    p.ask({ workspaceId: "W1", kind: "sprint" });
    expect(p.take("W2", "sprint")).toBe(false);
    expect(get(p.request)).toEqual({ workspaceId: "W1", kind: "sprint" });
  });

  it("keeps the request when the kind differs", () => {
    const p = createPlanRequest();
    p.ask({ workspaceId: "W1", kind: "sprint" });
    expect(p.take("W1", "release")).toBe(false);
    expect(get(p.request)).not.toBeNull();
  });

  it("clears the request when the same workspace and kind take", () => {
    const p = createPlanRequest();
    p.ask({ workspaceId: "W1", kind: "release" });
    expect(p.take("W1", "release")).toBe(true);
    expect(get(p.request)).toBeNull();
    expect(p.take("W1", "release")).toBe(false);
  });
});
