export type SseMessage = { event: string; data: string };

// parseEventStream returns the function to feed the chunks of a
// text/event-stream body to. It calls onMessage for each event, once its
// blank line has come. A line may be split between chunks; lines end with
// LF, CRLF or CR; several data: lines join with LF; lines that begin with a
// colon (the server's ": ping") are comments and are ignored.
export function parseEventStream(onMessage: (m: SseMessage) => void): (chunk: string) => void {
  let buf = "";
  let event = "";
  let data: string[] = [];

  const line = (l: string) => {
    if (l === "") {
      if (data.length > 0) onMessage({ event: event || "message", data: data.join("\n") });
      event = "";
      data = [];
      return;
    }
    if (l.startsWith(":")) return;
    const i = l.indexOf(":");
    const field = i < 0 ? l : l.slice(0, i);
    let value = i < 0 ? "" : l.slice(i + 1);
    if (value.startsWith(" ")) value = value.slice(1);
    if (field === "event") event = value;
    else if (field === "data") data.push(value);
    // id: and retry: are not used; the connection has its own backoff.
  };

  return (chunk) => {
    buf += chunk;
    let start = 0;
    for (let i = 0; i < buf.length; i++) {
      const c = buf[i];
      if (c !== "\n" && c !== "\r") continue;
      if (c === "\r" && i === buf.length - 1) break; // a "\n" may follow in the next chunk
      line(buf.slice(start, i));
      if (c === "\r" && buf[i + 1] === "\n") i++;
      start = i + 1;
    }
    buf = buf.slice(start);
  };
}
