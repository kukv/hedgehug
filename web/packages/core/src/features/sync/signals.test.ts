import { describe, expect, it } from "vitest";
import { fakeSyncPort } from "./fixtures.test-util";
import { createTabSignals } from "./signals";

function setup() {
  const port = fakeSyncPort();
  const st = { held: false, visible: true, online: true, now: 0 };
  const s = createTabSignals({
    port,
    tab: "T",
    held: () => st.held,
    visible: () => st.visible,
    online: () => st.online,
    now: () => st.now,
  });
  return { port, st, s };
}

describe("hold (決定 S3・C2-18)", () => {
  it("sends hold when held and visible, resends every 10 seconds, and releases", () => {
    const { port, st, s } = setup();
    s.check();
    expect(port.calls).toEqual([]);
    st.held = true;
    s.check();
    st.now = 9_999;
    s.check();
    st.now = 10_000;
    s.check();
    st.held = false;
    s.check();
    s.check();
    expect(port.calls).toEqual(["hold T true", "hold T true", "hold T false"]);
  });

  it("releases at once when the tab is hidden, and does not send while hidden", () => {
    const { port, st, s } = setup();
    st.held = true;
    s.check();
    st.visible = false;
    s.check();
    st.now = 30_000;
    s.check();
    expect(port.calls).toEqual(["hold T true", "hold T false"]);
  });
});

describe("network (決定 S4・C2-18b)", () => {
  it("sends offline at once and every 10 seconds, even when hidden, and online once", () => {
    const { port, st, s } = setup();
    st.visible = false;
    st.online = false;
    s.check();
    st.now = 5_000;
    s.check();
    st.now = 10_000;
    s.check();
    st.online = true;
    s.check();
    st.now = 60_000;
    s.check();
    expect(port.calls).toEqual(["network T false", "network T false", "network T true"]);
  });

  it("does not send online when it has never been offline", () => {
    const { port, s } = setup();
    s.check();
    expect(port.calls).toEqual([]);
  });

  it("ignores a failed request and sends again on the next resend", async () => {
    const { st } = setup();
    const failing = fakeSyncPort({
      network: async () => {
        throw new Error("down");
      },
    });
    const t = createTabSignals({
      port: failing,
      tab: "T",
      held: () => false,
      visible: () => true,
      online: () => false,
      now: () => st.now,
    });
    t.check();
    await Promise.resolve();
    st.now = 10_000;
    t.check();
    expect(failing.calls).toEqual(["network T false", "network T false"]);
  });
});
