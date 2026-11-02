import { cleanup, fireEvent } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import CreateStore from "./CreateStore.svelte";

const created = {
  name: "me",
  path: "/home/u/.local/share/hedgehug/stores/me",
  workspace: {
    ulid: "01HWSME",
    name: "me",
    id_prefix: "ME",
    prefix_set: "2026-11-02T00:00:00Z",
    tasks: 0,
  },
  remotes: [],
  pushed: [],
};

function mount(post = vi.fn().mockResolvedValue({ data: created, response: new Response() })) {
  const reload = vi.fn().mockResolvedValue(undefined);
  const onclose = vi.fn();
  const r = renderWithCore(
    CreateStore,
    { open: true, onclose },
    { api: { POST: post }, stores: { reload, subscribe: () => () => {} } },
  );
  return { post, reload, onclose, ...r };
}

const type = (el: HTMLElement, value: string) => fireEvent.input(el, { target: { value } });

beforeEach(() => {
  history.replaceState(null, "", "/");
  resetToasts();
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true,
  });
});
afterEach(cleanup);

describe("CreateStore", () => {
  it("名前と接頭辞が空なら送らず、理由を出す", async () => {
    const { getByRole, getByText, post } = mount();
    await fireEvent.click(getByRole("button", { name: "Store を作る" }));
    expect(post).not.toHaveBeenCalled();
    expect(getByText("名前を入れてください")).toBeTruthy();
    expect(getByText("接頭辞は英大文字と数字の 2〜10 文字で、英字から始めてください")).toBeTruthy();
  });

  it("接頭辞は大文字にそろえ、形が違えば送らない", async () => {
    const { getByLabelText, getByRole, post } = mount();
    const prefix = getByLabelText("ID の接頭辞") as HTMLInputElement;
    await type(prefix, "me");
    expect(prefix.value).toBe("ME");
    await type(getByLabelText("名前"), "me");
    await type(prefix, "1A");
    await fireEvent.click(getByRole("button", { name: "Store を作る" }));
    expect(post).not.toHaveBeenCalled();
  });

  it("作成: POST /api/stores、reload、トースト、最初のワークスペースのツリーへ", async () => {
    const { getByLabelText, getByRole, post, reload, onclose } = mount();
    await type(getByLabelText("名前"), " me ");
    await type(getByLabelText("ID の接頭辞"), "me");
    await fireEvent.click(getByRole("button", { name: "Store を作る" }));
    await vi.waitFor(() => expect(onclose).toHaveBeenCalledTimes(1));
    expect(post).toHaveBeenCalledWith("/api/stores", {
      body: { name: "me", prefix: "ME", workspace: undefined, remotes: undefined },
    });
    expect(reload).toHaveBeenCalledTimes(1);
    expect(toasts.list.map((t) => t.message)).toEqual(["Store「me」を作りました"]);
    expect(location.pathname).toBe("/workspaces/me/01HWSME");
  });

  it("最初のワークスペースと remote を渡す", async () => {
    const { getByLabelText, getByRole, post } = mount();
    await type(getByLabelText("名前"), "team");
    await type(getByLabelText("ID の接頭辞"), "TM");
    await type(getByLabelText("最初のワークスペース（任意）"), "backend");
    await type(getByLabelText("remote（任意）"), "git@github.com:org/hh-team.git");
    await fireEvent.click(getByRole("button", { name: "Store を作る" }));
    await vi.waitFor(() => expect(post).toHaveBeenCalledTimes(1));
    expect(post).toHaveBeenCalledWith("/api/stores", {
      body: {
        name: "team",
        prefix: "TM",
        workspace: "backend",
        remotes: ["git@github.com:org/hh-team.git"],
      },
    });
  });

  it("Enter（入力欄の中）でも作成する", async () => {
    const { getByLabelText, post } = mount();
    await type(getByLabelText("名前"), "me");
    const prefix = getByLabelText("ID の接頭辞");
    await type(prefix, "me");
    await fireEvent.keyDown(prefix, { key: "Enter" });
    await vi.waitFor(() => expect(post).toHaveBeenCalledTimes(1));
  });

  it("失敗したらエラーの文言を出し、ダイアログは閉じない", async () => {
    const post = vi.fn().mockRejectedValue(new Error("boom"));
    const { getByLabelText, getByRole, onclose } = mount(post);
    await type(getByLabelText("名前"), "me");
    await type(getByLabelText("ID の接頭辞"), "ME");
    await fireEvent.click(getByRole("button", { name: "Store を作る" }));
    const alert = await vi.waitFor(() => getByRole("alert"));
    expect(alert.textContent).toContain("操作に失敗しました: boom");
    expect(onclose).not.toHaveBeenCalled();
    expect((getByRole("button", { name: "Store を作る" }) as HTMLButtonElement).disabled).toBe(
      false,
    );
  });

  it("足元に同じ操作の hh コマンドを出す", async () => {
    const { getByLabelText, getByRole } = mount();
    await type(getByLabelText("名前"), "me");
    await type(getByLabelText("ID の接頭辞"), "me");
    expect(getByRole("button", { name: /hh store init me/ }).textContent).toContain("ME");
  });

  it("変換中（isComposing）の Enter では作成しない", async () => {
    const { getByLabelText, post } = mount();
    await type(getByLabelText("名前"), "me");
    const prefix = getByLabelText("ID の接頭辞");
    await type(prefix, "me");
    await fireEvent.keyDown(prefix, { key: "Enter", isComposing: true });
    expect(post).not.toHaveBeenCalled();
  });

  it("作成できたあとに stores.reload が失敗しても、作成の成功として扱う", async () => {
    const post = vi.fn().mockResolvedValue({ data: created, response: new Response() });
    const reload = vi.fn().mockRejectedValue(new Error("network"));
    const onclose = vi.fn();
    const { getByLabelText, getByRole, queryByRole } = renderWithCore(
      CreateStore,
      { open: true, onclose },
      { api: { POST: post }, stores: { reload, subscribe: () => () => {} } },
    );
    await type(getByLabelText("名前"), "me");
    await type(getByLabelText("ID の接頭辞"), "me");
    await fireEvent.click(getByRole("button", { name: "Store を作る" }));
    await vi.waitFor(() => expect(onclose).toHaveBeenCalledTimes(1));
    expect(reload).toHaveBeenCalledTimes(1);
    expect(queryByRole("alert")).toBeNull();
    expect(toasts.list.map((t) => t.message)).toEqual(["Store「me」を作りました"]);
    expect(location.pathname).toBe("/workspaces/me/01HWSME");
  });

  it("キャンセルすると失敗の表示を消す", async () => {
    const post = vi.fn().mockRejectedValue(new Error("boom"));
    const { getByLabelText, getByRole, queryByRole, onclose } = mount(post);
    await type(getByLabelText("名前"), "me");
    await type(getByLabelText("ID の接頭辞"), "ME");
    await fireEvent.click(getByRole("button", { name: "Store を作る" }));
    await vi.waitFor(() => getByRole("alert"));
    await fireEvent.click(getByRole("button", { name: "キャンセル" }));
    expect(onclose).toHaveBeenCalledTimes(1);
    expect(queryByRole("alert")).toBeNull();
  });

  it("作成中はキャンセルできず、閉じない", async () => {
    let finish: (v: unknown) => void = () => {};
    const post = vi.fn().mockReturnValue(new Promise((r) => (finish = r)));
    const { getByLabelText, getByRole, onclose } = mount(post);
    await type(getByLabelText("名前"), "me");
    await type(getByLabelText("ID の接頭辞"), "ME");
    await fireEvent.click(getByRole("button", { name: "Store を作る" }));
    const cancel = getByRole("button", { name: "キャンセル" }) as HTMLButtonElement;
    expect(cancel.disabled).toBe(true);
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(onclose).not.toHaveBeenCalled();
    finish({ data: created, response: new Response() });
    await vi.waitFor(() => expect(onclose).toHaveBeenCalledTimes(1));
  });
});
