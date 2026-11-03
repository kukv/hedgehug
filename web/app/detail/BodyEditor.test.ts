import { beforeAll, describe, expect, it, vi } from "vitest";
import BodyEditor from "./BodyEditor.svelte";
import { makeCore, mount, stubLayout, viewOf } from "./testkit";

const props = (over: Record<string, unknown> = {}) => ({
  value: "# title",
  store: "shared",
  pending: false,
  onedit: vi.fn(),
  oncommit: vi.fn(),
  ...over,
});

describe("BodyEditor", () => {
  beforeAll(stubLayout);

  it("starts with the value, labelled for assistive technology", () => {
    const { container } = mount(BodyEditor, props(), makeCore());
    expect(viewOf(container).state.doc.toString()).toBe("# title");
    expect(container.querySelector('[aria-label="本文"]')).not.toBeNull();
  });

  it("reports edits made by the user", () => {
    const onedit = vi.fn();
    const { container } = mount(BodyEditor, props({ value: "a", onedit }), makeCore());
    viewOf(container).dispatch({ changes: { from: 1, insert: "b" } });
    expect(onedit).toHaveBeenLastCalledWith("ab");
  });

  it("commits when the editor loses focus", () => {
    const oncommit = vi.fn();
    const { container } = mount(BodyEditor, props({ oncommit }), makeCore());
    viewOf(container).contentDOM.dispatchEvent(new FocusEvent("blur"));
    expect(oncommit).toHaveBeenCalledTimes(1);
  });

  it("takes a change from outside unless a save is pending, without reporting it as an edit", async () => {
    const onedit = vi.fn();
    const { container, rerender } = mount(BodyEditor, props({ value: "one", onedit }), makeCore());
    const view = viewOf(container);
    await rerender({ value: "two", pending: true });
    expect(view.state.doc.toString()).toBe("one");
    await rerender({ value: "two", pending: false });
    expect(view.state.doc.toString()).toBe("two");
    expect(onedit).not.toHaveBeenCalled();
  });

  it("setText and getText read and replace the document without an edit", () => {
    const onedit = vi.fn();
    const { container, component } = mount(
      BodyEditor,
      props({ value: "- [ ] a", onedit }),
      makeCore(),
    );
    const editor = component as unknown as { getText(): string; setText(t: string): void };
    expect(editor.getText()).toBe("- [ ] a");
    editor.setText("- [x] a");
    expect(viewOf(container).state.doc.toString()).toBe("- [x] a");
    expect(onedit).not.toHaveBeenCalled();
  });

  it("does not accept edits when read-only", () => {
    const { container } = mount(BodyEditor, props({ readonly: true }), makeCore());
    expect(viewOf(container).state.readOnly).toBe(true);
  });

  it("offers the people for @ from GET /api/people", async () => {
    const k = makeCore();
    k.api.GET.mockImplementation(() =>
      Promise.resolve({ data: ["suzuki", "tanaka"], response: new Response() }),
    );
    const { container } = mount(BodyEditor, props({ value: "" }), k);
    const view = viewOf(container);
    // The completion starts 100ms after the typing (CodeMirror's activateOnTypingDelay). The clock
    // is a fake one: on a busy machine the real 100ms ran past the 1s of waitFor.
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    try {
      view.dispatch({
        changes: { from: 0, insert: "@su" },
        selection: { anchor: 3 },
        userEvent: "input.type",
      });
      await vi.advanceTimersByTimeAsync(150);
      expect(k.api.GET).toHaveBeenCalledWith("/api/people", {
        params: { query: { store: "shared" } },
      });
    } finally {
      vi.useRealTimers();
    }
  });
});
