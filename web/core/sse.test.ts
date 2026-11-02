import { describe, expect, it } from "vitest";
import { parseEventStream, type SseMessage } from "./sse";

function collect() {
  const got: SseMessage[] = [];
  return { got, feed: parseEventStream((m) => got.push(m)) };
}

describe("parseEventStream", () => {
  it("reads an event with its name and data", () => {
    const { got, feed } = collect();
    feed('event: hello\ndata: {"client":"c1"}\n\n');
    expect(got).toEqual([{ event: "hello", data: '{"client":"c1"}' }]);
  });

  it("joins a line that is split between chunks", () => {
    const { got, feed } = collect();
    feed("event: tasks.cha");
    feed('nged\ndata: {"sto');
    feed('re":"s"}\n');
    expect(got).toEqual([]);
    feed("\n");
    expect(got).toEqual([{ event: "tasks.changed", data: '{"store":"s"}' }]);
  });

  it("reads CRLF and a CRLF split between chunks", () => {
    const { got, feed } = collect();
    feed("event: a\r\ndata: 1\r");
    feed("\n\r\n");
    feed("event: b\rdata: 2\r\revent: c\n");
    expect(got).toEqual([
      { event: "a", data: "1" },
      { event: "b", data: "2" },
    ]);
  });

  it("joins several data lines with LF", () => {
    const { got, feed } = collect();
    feed("event: x\ndata: a\ndata: b\ndata:c\n\n");
    expect(got).toEqual([{ event: "x", data: "a\nb\nc" }]);
  });

  it("ignores comment lines such as : ping", () => {
    const { got, feed } = collect();
    feed(": ping\n\n");
    feed("event: x\n: ping\ndata: 1\n\n");
    expect(got).toEqual([{ event: "x", data: "1" }]);
  });

  it("names an event with no event: line message, and drops one with no data", () => {
    const { got, feed } = collect();
    feed("data: 1\n\n");
    feed("event: lonely\n\n");
    feed("id: 7\nretry: 1000\ndata: 2\n\n");
    expect(got).toEqual([
      { event: "message", data: "1" },
      { event: "message", data: "2" },
    ]);
  });

  it("reads several events in one chunk", () => {
    const { got, feed } = collect();
    feed("event: a\ndata: 1\n\nevent: b\ndata: 2\n\n");
    expect(got.map((m) => m.event)).toEqual(["a", "b"]);
  });
});
