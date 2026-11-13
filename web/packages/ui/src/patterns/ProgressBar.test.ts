import { render, screen } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import ProgressBar from "./ProgressBar.svelte";

describe("ProgressBar", () => {
  it("shows the bar and N%, with 完了 / 対象 as the title of the bar", () => {
    const { container } = render(ProgressBar, { props: { percent: 38, detail: "3 / 8" } });
    const bar = screen.getByRole("progressbar", { name: "進捗" });
    expect(bar).toHaveAttribute("title", "3 / 8");
    expect(bar).toHaveAttribute("aria-valuenow", "38");
    expect(bar).toHaveAttribute("aria-valuetext", "3 / 8");
    expect(screen.getByText("38%")).toBeInTheDocument();
    expect(container.querySelector(".fill")).toHaveStyle({ width: "38%" });
  });

  it("is empty at 0%", () => {
    const { container } = render(ProgressBar, { props: { percent: 0, detail: "0 / 0" } });
    expect(screen.getByText("0%")).toBeInTheDocument();
    expect(container.querySelector(".fill")).toHaveStyle({ width: "0%" });
  });
});
