import { describe, expect, it } from "vitest";
import { projectCategory } from "./status";

describe("projectCategory", () => {
  it("maps the five statuses to the four categories", () => {
    expect(projectCategory("Todo")).toBe("todo");
    expect(projectCategory("Pending")).toBe("todo");
    expect(projectCategory("InProgress")).toBe("doing");
    expect(projectCategory("Done")).toBe("done");
    expect(projectCategory("Cancel")).toBe("canceled");
  });
});
