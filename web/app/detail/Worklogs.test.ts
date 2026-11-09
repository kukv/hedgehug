import { screen } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import { makeCore, mount, ok, showView } from "./testkit";
import Worklogs from "./Worklogs.svelte";

function setup(over: Parameters<typeof showView>[0]) {
  const k = makeCore();
  k.api.listWorklogs.mockImplementation(() => ok([]));
  return mount(Worklogs, { show: showView(over) }, k);
}

describe("Worklogs", () => {
  it("shows the time spent against the estimate, a field of the task", () => {
    setup({ estimate: "6h", worklog: { count: 1, total: "2h" } });
    expect(screen.getByText("2h / 見積もり 6h")).toBeInTheDocument();
  });

  it("shows the time alone for a task with no estimate, and does not take props.estimate for one", () => {
    setup({ estimate: null, props: { estimate: "9h" }, worklog: { count: 1, total: "2h" } });
    expect(screen.getByText("2h")).toBeInTheDocument();
  });
});
