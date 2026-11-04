import { cleanup, fireEvent, screen } from "@testing-library/svelte";
import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AuthDeviceEvent } from "../../core/connection";
import { renderWithCore } from "../test-utils";
import DeviceFlow from "./DeviceFlow.svelte";

const NOW = new Date("2026-11-04T09:00:00Z");
const flow = {
  id: "F1",
  user_code: "ABCD-1234",
  verification_uri: "https://github.com/login/device",
  expires_at: "2026-11-04T09:14:59Z",
};

function mount(over: { resumed?: boolean } = {}) {
  const handlers = new Set<(e: AuthDeviceEvent) => void>();
  const conn = {
    on: (_: string, fn: (e: AuthDeviceEvent) => void) => {
      handlers.add(fn);
      return () => handlers.delete(fn);
    },
  };
  const ondone = vi.fn();
  const onretry = vi.fn();
  renderWithCore(DeviceFlow, { flow, ondone, onretry, ...over }, { conn });
  const emit = (e: AuthDeviceEvent) => {
    for (const fn of [...handlers]) fn(e);
    flushSync();
  };
  return { ondone, onretry, emit, handlers };
}

const writeText = vi.fn().mockResolvedValue(undefined);

beforeEach(() => {
  vi.useFakeTimers({
    toFake: ["setInterval", "clearInterval", "setTimeout", "clearTimeout", "Date"],
  });
  vi.setSystemTime(NOW);
  writeText.mockClear();
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("DeviceFlow", () => {
  it("shows the code, the link to GitHub in a new tab and the time left", () => {
    mount();
    expect(screen.getByText("このコードを GitHub で入力してください")).toBeTruthy();
    expect(screen.getByText("ABCD-1234")).toBeTruthy();
    const a = screen.getByRole("link", { name: "github.com/login/device を開く" });
    expect(a.getAttribute("href")).toBe("https://github.com/login/device");
    expect(a.getAttribute("target")).toBe("_blank");
    expect(a.getAttribute("rel")).toBe("noopener noreferrer");
    expect(screen.getByText("承認を待っています · コードの期限まで 14:59")).toBeTruthy();
  });

  it("counts down every second", async () => {
    mount();
    await vi.advanceTimersByTimeAsync(61_000);
    expect(screen.getByText("承認を待っています · コードの期限まで 13:58")).toBeTruthy();
  });

  it("draws no link for an address that is not a web address", () => {
    renderWithCore(
      DeviceFlow,
      {
        flow: { ...flow, verification_uri: "javascript:alert(1)" },
        ondone: vi.fn(),
        onretry: vi.fn(),
      },
      { conn: { on: () => () => {} } },
    );
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("copies the code", async () => {
    mount();
    await fireEvent.click(screen.getByRole("button", { name: "コピー" }));
    expect(writeText).toHaveBeenCalledWith("ABCD-1234");
    expect(screen.getByRole("button", { name: "コピーしました" })).toBeTruthy();
  });

  it("says when it carries on a flow that another screen started", () => {
    mount({ resumed: true });
    expect(screen.getByText("別の画面で始めたログインを続けます")).toBeTruthy();
  });

  it("tells the parent the account when the flow is done, and only for its own flow", () => {
    const { ondone, emit } = mount();
    emit({ id: "other", state: "done", account: "x" });
    expect(ondone).not.toHaveBeenCalled();
    emit({ id: "F1", state: "done", account: "work" });
    expect(ondone).toHaveBeenCalledWith("work");
  });

  it("shows the words of an error event and offers to start again", async () => {
    const { onretry, emit } = mount();
    emit({ id: "F1", state: "error", code: "auth.canceled", message: "canceled" });
    expect(screen.getByRole("alert").textContent).toBe("ログインをやめました");
    expect(screen.queryByText("このコードを GitHub で入力してください")).toBeNull();
    await fireEvent.click(screen.getByRole("button", { name: "やり直す" }));
    expect(onretry).toHaveBeenCalledTimes(1);
  });

  it("says the code expired when the time is up", async () => {
    const { onretry } = mount();
    await vi.advanceTimersByTimeAsync(15 * 60 * 1000);
    expect(screen.getByRole("alert").textContent).toBe("コードの期限が切れました");
    expect(screen.queryByText("ABCD-1234")).toBeNull();
    await fireEvent.click(screen.getByRole("button", { name: "やり直す" }));
    expect(onretry).toHaveBeenCalledTimes(1);
  });

  it("stops listening and counting when it goes", () => {
    const { handlers } = mount();
    expect(handlers.size).toBe(1);
    cleanup();
    expect(handlers.size).toBe(0);
    expect(vi.getTimerCount()).toBe(0);
  });
});
