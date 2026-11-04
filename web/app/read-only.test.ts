import { beforeEach, describe, expect, it } from "vitest";
import { readOnlySentence, readOnlyToast, useReadOnly } from "./read-only";
import { resetToasts, toasts } from "./toast.svelte";

function fakeStores() {
  let why: string | null = null;
  return {
    readOnly: () => why,
    subscribe: () => () => {},
    set(v: string | null) {
      why = v;
    },
  };
}

describe("useReadOnly", () => {
  it("writes the reason in Japanese, and goes back to null when the Store can be written", () => {
    const s = fakeStores();
    const r = useReadOnly(s, () => "ro");
    expect(r.current).toBeNull();
    s.set("the remote cannot be written");
    expect(r.current).toBe("remote に書き込めません");
    s.set(null);
    expect(r.current).toBeNull();
  });

  it("is null without a store", () => {
    const s = fakeStores();
    s.set("the GitHub permission is read");
    expect(useReadOnly(s, () => undefined).current).toBeNull();
  });
});

describe("readOnlyToast", () => {
  beforeEach(resetToasts);

  it("says the Store is read-only, with the reason", () => {
    readOnlyToast("remote に書き込めません");
    expect(toasts.list[0]?.message).toBe("この Store は閲覧のみです（remote に書き込めません）");
    expect(readOnlySentence("x")).toBe("この Store は閲覧のみです（x）");
  });
});
