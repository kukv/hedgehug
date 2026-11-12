import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createShell } from "./shell";

describe("shell", () => {
  it("opens, closes and toggles the drawer", () => {
    const shell = createShell();
    expect(get(shell.drawerOpen)).toBe(false);
    shell.openDrawer();
    expect(get(shell.drawerOpen)).toBe(true);
    shell.closeDrawer();
    expect(get(shell.drawerOpen)).toBe(false);
    shell.toggleDrawer();
    expect(get(shell.drawerOpen)).toBe(true);
  });
});
