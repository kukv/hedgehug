import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";
import { afterEach, describe, expect, it } from "vitest";
import ColorPicker from "./ColorPicker.svelte";

afterEach(cleanup);

// A bound value: the component writes the choice to it.
function mount(initial: string, original = initial) {
  const box = { value: initial };
  render(ColorPicker, {
    props: {
      label: "色",
      original,
      get value() {
        return box.value;
      },
      set value(v: string) {
        box.value = v;
      },
    },
  });
  return box;
}

const free = () => screen.getByLabelText("色を選ぶ") as HTMLInputElement;

describe("ColorPicker の自由な色", () => {
  it("自由な色を選ぶと #rrggbb を返す", async () => {
    const box = mount("");
    expect(screen.getByRole("radio", { name: "自動" })).toBeChecked();
    await fireEvent.input(free(), { target: { value: "#3451d1" } });
    expect(box.value).toBe("#3451d1");
  });

  it("12 色にない #hex は自由な色を選択中にして値を入れる", () => {
    const box = mount("#3451D1");
    expect(screen.getByRole("radio", { name: "自由な色" })).toBeChecked();
    expect(free().value).toBe("#3451d1");
    expect(box.value).toBe("#3451D1");
    expect(screen.queryByRole("radio", { name: /カスタム/ })).toBeNull();
  });

  it("12 色のどれかを選ぶと自由な色から外れる", async () => {
    const box = mount("#3451D1");
    await fireEvent.click(screen.getByRole("radio", { name: "ティール" }));
    expect(box.value).toBe("teal");
    expect(screen.getByRole("radio", { name: "自由な色" })).not.toBeChecked();
  });

  it("hex でも名前でもない値は今までどおりカスタムで保持する", () => {
    const box = mount("star");
    expect(screen.getByRole("radio", { name: "カスタム（star）" })).toBeChecked();
    expect(box.value).toBe("star");
  });
});
