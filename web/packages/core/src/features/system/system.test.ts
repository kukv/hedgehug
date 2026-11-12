import { describe, expect, it } from "vitest";
import type { SystemPort } from "../../port/system";
import { get } from "../../reactive/store";
import { createSystemInfo } from "./system";

describe("system info", () => {
  it("loads the version from the server", async () => {
    const port: SystemPort = { getSystem: async () => ({ version: "v1.2.3" }) };
    const info = createSystemInfo(port);
    expect(get(info.version)).toBeNull();
    expect(await info.load()).toBe(true);
    expect(get(info.version)).toBe("v1.2.3");
  });

  it("keeps the version empty when the server fails", async () => {
    const port: SystemPort = {
      getSystem: async () => {
        throw new Error("401");
      },
    };
    const info = createSystemInfo(port);
    expect(await info.load()).toBe(false);
    expect(get(info.version)).toBeNull();
  });
});
