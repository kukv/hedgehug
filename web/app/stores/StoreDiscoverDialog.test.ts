import { cleanup, fireEvent, screen, waitFor } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import StoreDiscoverDialog from "./StoreDiscoverDialog.svelte";

const found = ["hh-a", "hh-b"].map((name) => ({
  name,
  url: `https://github.com/org/${name}.git`,
  account: "me",
  permission: "write",
}));

function mount(result: unknown) {
  const api = { discoverStores: vi.fn(() => ok(found)), mountStores: vi.fn(() => ok(result)) };
  const accounts = {
    accounts: () => [
      { name: "me", id: "github.com/me", state: "ok", login: "me", token: "keychain" },
    ],
    loaded: () => true,
    error: () => null,
    load: vi.fn(async () => {}),
    reload: vi.fn(async () => {}),
    subscribe: () => () => {},
  };
  const onclose = vi.fn();
  renderWithCore(
    StoreDiscoverDialog,
    { open: true, onclose },
    { api, accounts, stores: { reload: vi.fn(async () => {}), subscribe: () => () => {} } },
  );
  return { onclose };
}

const mountBoth = async () => {
  await fireEvent.click(await screen.findByLabelText("org/hh-a を選ぶ"));
  await fireEvent.click(screen.getByLabelText("org/hh-b を選ぶ"));
  await fireEvent.click(screen.getByRole("button", { name: "マウント" }));
};

beforeEach(() => {
  resetToasts();
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true,
  });
});
afterEach(() => cleanup());

describe("StoreDiscoverDialog", () => {
  it("is titled Store を探す", () => {
    mount({ mounted: [], failed: [] });
    expect(screen.getByRole("dialog", { name: "Store を探す" })).toBeTruthy();
  });

  it("toasts how many were mounted and closes", async () => {
    const { onclose } = mount({ mounted: ["hh-a", "hh-b"], failed: [] });
    await mountBoth();
    await waitFor(() => expect(onclose).toHaveBeenCalledTimes(1));
    expect(toasts.list.map((t) => t.message)).toEqual(["2 個の Store をマウントしました"]);
  });

  it("stays open while a row failed", async () => {
    const { onclose } = mount({
      mounted: ["hh-a"],
      failed: [{ name: "hh-b", code: "store.exists", message: "used" }],
    });
    await mountBoth();
    await screen.findByText("hh-b という Store はすでにあります");
    expect(onclose).not.toHaveBeenCalled();
    expect(toasts.list.map((t) => t.message)).toEqual(["1 個の Store をマウントしました"]);
  });
});
