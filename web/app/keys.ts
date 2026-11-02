export function isMac(): boolean {
  return /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);
}

/** ⌘ on a Mac, Ctrl elsewhere. */
export function modKey(e: KeyboardEvent): boolean {
  return isMac() ? e.metaKey : e.ctrlKey;
}

type Combo = { key: string; mod: boolean; shift: boolean; alt: boolean };

function parse(combo: string): Combo {
  const parts = combo.toLowerCase().split("+");
  return {
    key: parts.at(-1) ?? "",
    mod: parts.includes("mod"),
    shift: parts.includes("shift"),
    alt: parts.includes("alt"),
  };
}

function keyOf(e: KeyboardEvent): string {
  return e.key === " " ? "space" : e.key.toLowerCase();
}

function matches(e: KeyboardEvent, c: Combo): boolean {
  const mac = isMac();
  const mod = mac ? e.metaKey : e.ctrlKey;
  const other = mac ? e.ctrlKey : e.metaKey;
  return (
    keyOf(e) === c.key && mod === c.mod && !other && e.shiftKey === c.shift && e.altKey === c.alt
  );
}

function editable(t: EventTarget | null): boolean {
  const el = t as HTMLElement | null;
  if (!el?.tagName) return false;
  return el.isContentEditable === true || ["INPUT", "TEXTAREA", "SELECT"].includes(el.tagName);
}

type Entry = { combo: Combo; fn: (e: KeyboardEvent) => void };
const entries = new Set<Entry>();

function onKeydown(e: KeyboardEvent): void {
  if (e.isComposing) return;
  // A full-screen gate (no token, no server, no store) stops every shortcut, ⌘/Ctrl ones too.
  if (document.querySelector("[data-blocking]")) return;
  for (const en of entries) {
    if (!matches(e, en.combo)) continue;
    // A key without ⌘/Ctrl is for typing in a field or a dialog.
    if (!en.combo.mod && (editable(e.target) || document.querySelector('[aria-modal="true"]'))) {
      continue;
    }
    e.preventDefault();
    en.fn(e);
    return;
  }
}

/** Registers a shortcut such as "mod+k", "c" or "shift+mod+s"; returns the function that removes it. */
export function registerShortcut(combo: string, fn: (e: KeyboardEvent) => void): () => void {
  const entry: Entry = { combo: parse(combo), fn };
  if (entries.size === 0) window.addEventListener("keydown", onKeydown);
  entries.add(entry);
  return () => {
    entries.delete(entry);
    if (entries.size === 0) window.removeEventListener("keydown", onKeydown);
  };
}

const symbols: Record<string, string> = {
  enter: "↵",
  arrowup: "↑",
  arrowdown: "↓",
  space: "Space",
  tab: "Tab",
};

/** "mod+k" as the label of the key: ⌘K on a Mac, Ctrl+K elsewhere. */
export function kbdLabel(combo: string): string {
  const c = parse(combo);
  const key = symbols[c.key] ?? c.key.toUpperCase();
  if (isMac()) return `${c.alt ? "⌥" : ""}${c.shift ? "⇧" : ""}${c.mod ? "⌘" : ""}${key}`;
  return [c.mod && "Ctrl", c.alt && "Alt", c.shift && "Shift", key].filter(Boolean).join("+");
}
