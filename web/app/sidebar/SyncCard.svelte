<script lang="ts">
  import { onMount } from "svelte";
  import { formatRelative } from "../../core/format";
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import { kbdLabel, registerShortcut } from "../keys";
  import { online } from "../online";
  import { useSubscribe } from "../svelte-store";
  import { runSync, syncRun } from "./sync-run.svelte";
  import { summarizeSync } from "./sync-summary";

  // store: the card of one store (the settings); without it the card folds every store and owns the key.
  let { store }: { store?: string } = $props();

  const { api, conn } = core();
  const statuses = useSubscribe(conn, () => conn.sync);
  const summary = $derived(
    summarizeSync(
      [...statuses.current.values()].filter((s) => store === undefined || s.store === store),
      online.current,
    ),
  );

  let now = $state(Date.now());

  // Only what is drawn from the clock (countdowns, "n minutes ago") needs it to move.
  const ticking = $derived(
    (summary.kind === "synced" && (summary.nextFetch !== null || summary.lastSuccess !== null)) ||
      (summary.kind === "pending" && summary.pushAt !== null),
  );

  $effect(() => {
    if (!ticking) return;
    now = Date.now();
    const tick = setInterval(() => {
      now = Date.now();
    }, 1000);
    return () => clearInterval(tick);
  });

  onMount(() => {
    if (store === undefined) return registerShortcut("shift+mod+s", () => void syncNow());
  });

  const pad = (n: number) => String(n).padStart(2, "0");

  function clock(iso: string): string {
    const d = new Date(iso);
    return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }

  function countdown(iso: string): string {
    const s = Math.max(0, Math.floor((Date.parse(iso) - now) / 1000));
    return s === 0 ? "まもなく" : `${Math.floor(s / 60)}:${pad(s % 60)}`;
  }

  function pushIn(iso: string): string {
    const s = Math.floor((Date.parse(iso) - now) / 1000);
    if (s <= 0) return "まもなく push";
    return s < 60 ? `${s} 秒後に push` : `${Math.ceil(s / 60)} 分後に push`;
  }

  const syncNow = () => runSync(api, store);
</script>

<div class="card">
  {#if summary.kind === "synced"}
    <div class="head">
      <span class="dot ok"></span>
      <b>同期済み</b>
      <span class="aside">
        {#if summary.noRemote}
          remote なし
        {:else if summary.lastSuccess}
          {formatRelative(summary.lastSuccess, new Date(now))}
        {/if}
      </span>
    </div>
    <div class="row"><span>未 push</span><span class="v">0 件</span></div>
    {#if summary.nextFetch}
      <div class="row">
        <span>次の fetch</span><span class="v mono">{countdown(summary.nextFetch)}</span>
      </div>
    {/if}
  {:else if summary.kind === "pending"}
    <div class="head">
      <span class="dot pending"></span>
      <b>送信待ち</b>
      {#if summary.pushAt}
        <span class="aside">{pushIn(summary.pushAt)}</span>
      {/if}
    </div>
    <div class="row"><span>未 push</span><span class="v">{summary.unpushed} コミット</span></div>
  {:else if summary.kind === "error"}
    <div class="head">
      <span class="dot bad"></span>
      <b class="bad-text">push できません</b>
    </div>
    {@const first = summary.errors[0]}
    <p class="msg">{first?.store}: {first?.message}</p>
    <div class="row"><span>未 push</span><span class="v">{summary.unpushed} コミット</span></div>
  {:else}
    <div class="head">
      <span class="dot off"></span>
      <b>オフライン</b>
      {#if summary.lastSuccess}
        <span class="aside">最終同期 {clock(summary.lastSuccess)}</span>
      {/if}
    </div>
    <div class="row"><span>未 push</span><span class="v">{summary.unpushed} コミット</span></div>
  {/if}
  <button
    type="button"
    class="now"
    disabled={syncRun.busy}
    onclick={syncNow}
    title={store === undefined ? kbdLabel("shift+mod+s") : undefined}
  >
    <Icon name="sync" size={14} />
    {syncRun.busy ? "同期中…" : "今すぐ同期"}
  </button>
</div>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding: var(--space-12);
    border: 1px solid var(--line-soft);
    border-radius: var(--radius-10);
    background: var(--surface);
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--space-8);
  }
  .head b {
    font-weight: 600;
  }
  .bad-text {
    color: var(--danger-ink);
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 999px;
  }
  .ok {
    background: var(--success);
  }
  .pending {
    background: var(--pending);
  }
  .bad {
    background: var(--danger);
  }
  .off {
    background: var(--muted-2);
  }
  .aside {
    margin-left: auto;
    color: var(--label);
    font-size: var(--fs-12);
  }
  .row {
    display: flex;
    justify-content: space-between;
    color: var(--label);
    font-size: var(--fs-12);
  }
  .v {
    color: var(--ink);
  }
  .mono {
    font-family: var(--font-mono);
  }
  .msg {
    margin: 0;
    color: var(--ink-2);
    font-size: var(--fs-12);
    line-height: 1.6;
    overflow-wrap: anywhere;
  }
  .now {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-6);
    min-height: 32px;
    border: 1px solid var(--line-input);
    border-radius: var(--radius-6);
    background: var(--surface);
    font-size: var(--fs-12);
    cursor: pointer;
  }
  .now:disabled {
    color: var(--muted);
    cursor: progress;
  }
</style>
