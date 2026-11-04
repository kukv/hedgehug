<script lang="ts">
  import { core } from "../context";
  import { useSubscribe } from "../svelte-store";
  import { copyText } from "./clipboard";

  // reveal: the owner shows the ULID (a row of the tree does while the keyboard is on it), to
  // read. The ID is not a stop of the Tab key: the row is.
  // href: the ID is a link (a task listed in a detail), and onopen is how the app follows it.
  type Props = {
    id: string;
    ulid: string;
    // the Store of the task: the ID is shown with the Store name when its prefix is in two Stores
    store: string;
    alwaysUlid?: boolean;
    reveal?: boolean;
    href?: string;
    onopen?: (e: MouseEvent) => void;
    // the middle button: the browser opens the link in a new tab by itself
    onaux?: (e: MouseEvent) => void;
  };
  let {
    id,
    ulid,
    store,
    alwaysUlid = false,
    reveal = false,
    href,
    onopen,
    onaux,
  }: Props = $props();

  const { stores } = core();
  const shown = useSubscribe(stores, () => stores.displayId(store, id));

  let hover = $state(false);
  let focus = $state(false);
  const show = $derived(hover || focus || reveal);
  let copied = $state(false);

  async function copy(e: MouseEvent) {
    e.stopPropagation();
    if (await copyText(ulid)) {
      copied = true;
      setTimeout(() => {
        copied = false;
      }, 1500);
    }
  }
</script>

<!-- biome-ignore lint/a11y/noStaticElementInteractions: hover and focus only reveal the ULID; the buttons inside are the interactive elements -->
<span
  class="idl"
  role="presentation"
  onmouseenter={() => (hover = true)}
  onmouseleave={() => (hover = false)}
  onfocusin={() => (focus = true)}
  onfocusout={(e) => {
    if (!e.currentTarget.contains(e.relatedTarget as Node | null)) focus = false;
  }}
  onclick={(e) => e.stopPropagation()}
  onkeydown={(e) => {
    // A link is a stop of the Tab key: the shortcuts of the window (C, ⌘K) keep working on it.
    if (!href) e.stopPropagation();
  }}
>
  {#if href}
    <a class="id link" {href} onclick={onopen} onauxclick={onaux}>{shown.current}</a>
  {:else}
    <button type="button" class="id" tabindex="-1">{shown.current}</button>
  {/if}
  {#if alwaysUlid}
    <span class="ulid">{ulid}</span>
  {:else if show}
    <span class="pop" role="tooltip">
      <span class="ulid">{ulid}</span>
      <button type="button" class="copy" onclick={copy} aria-label="ULID をコピー">
        {copied ? "コピーしました" : "コピー"}
      </button>
    </span>
  {/if}
</span>

<style>
  .idl {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: var(--space-8);
  }
  .id {
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
    cursor: default;
  }
  .link {
    text-decoration: none;
    cursor: pointer;
  }
  .ulid {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
  .pop {
    position: absolute;
    top: 100%;
    left: 0;
    z-index: 20;
    display: inline-flex;
    align-items: center;
    gap: var(--space-8);
    padding: var(--space-6) var(--space-8);
    border-radius: var(--radius-8);
    background: var(--surface);
    box-shadow: var(--shadow-popover);
    white-space: nowrap;
  }
  .copy {
    height: var(--control-h-sm);
    padding: 0 var(--space-8);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-6);
    background: var(--surface);
    font-size: var(--fs-12);
    cursor: pointer;
  }
</style>
