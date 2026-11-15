import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { fakeSettings, network } from "./fixtures.test-util";
import { createSyncTimingField } from "./general";

describe("createSyncTimingField", () => {
  it("loads the timing and saves a change", async () => {
    const f = fakeSettings();
    const timing = createSyncTimingField(f.port, f.toasts);
    expect(get(timing.state)).toBeNull();
    await timing.load();
    expect(get(timing.state)).toEqual({
      auto: true,
      intervalMinutes: 5,
      afterWriteSeconds: 30,
      githubMinutes: 5,
    });
    await timing.set({ intervalMinutes: 15 });
    expect(f.calls).toEqual([
      'timing {"auto":true,"intervalMinutes":15,"afterWriteSeconds":30,"githubMinutes":5}',
    ]);
    expect(get(timing.state)?.intervalMinutes).toBe(15);
  });

  it("saves overlapping changes one by one, keeping both", async () => {
    const f = fakeSettings();
    const timing = createSyncTimingField(f.port, f.toasts);
    await timing.load();
    f.ctl.hold = true;
    const first = timing.set({ intervalMinutes: 15 });
    const second = timing.set({ githubMinutes: 30 });
    expect(get(timing.state)).toMatchObject({ intervalMinutes: 15, githubMinutes: 30 });
    await Promise.resolve();
    f.ctl.hold = false;
    f.release();
    await Promise.all([first, second]);
    expect(f.calls).toEqual([
      'timing {"auto":true,"intervalMinutes":15,"afterWriteSeconds":30,"githubMinutes":5}',
      'timing {"auto":true,"intervalMinutes":15,"afterWriteSeconds":30,"githubMinutes":30}',
    ]);
    expect(await f.port.timing()).toMatchObject({ intervalMinutes: 15, githubMinutes: 30 });
    expect(get(timing.state)).toMatchObject({ intervalMinutes: 15, githubMinutes: 30 });
  });

  it("shows an error toast when the timing cannot be read", async () => {
    const f = fakeSettings();
    f.ctl.fail.add("timing");
    const timing = createSyncTimingField(f.port, f.toasts);
    await timing.load();
    expect(get(timing.state)).toBeNull();
    expect(f.errors()).toEqual([network]);
  });

  it("goes back to the saved timing when saving fails, with an error toast", async () => {
    const f = fakeSettings();
    const timing = createSyncTimingField(f.port, f.toasts);
    await timing.load();
    f.ctl.fail.add("timing");
    await timing.set({ auto: false });
    expect(get(timing.state)?.auto).toBe(true);
    expect(f.errors()).toEqual([network]);
  });
});
