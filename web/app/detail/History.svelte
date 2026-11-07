<script lang="ts">
  import { automationOf, opLabel, rejectionsOf } from "../../core/activity";
  import type { Schemas, ShowView } from "../../core/api/client";
  import { cliRestoreAt } from "../../core/cli";
  import { formatDateTime } from "../../core/format";
  import { eventLabel } from "../../core/github";
  import { diffForRestore, type RestoreChange, showAt } from "../../core/history";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { useDisplayId, useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import Dialog from "../ui/Dialog.svelte";

  type Log = Schemas.LogView;

  let { show, readonly = false }: { show: ShowView; readonly?: boolean } = $props();

  const { api, cache, schema, stores } = core();
  const did = useDisplayId(stores);
  const sc = useSubscribe(schema, () => schema.get(show.store));

  let logs = $state<Log[]>([]);
  let loadError = $state("");
  $effect(() => {
    // Any commit gives a new show (a comment does not move `updated`).
    void show;
    let live = true;
    api.getTaskHistory(show.ulid, { store: show.store, limit: 50 }).then(
      (r) => {
        if (live) {
          logs = r ?? [];
          loadError = "";
        }
      },
      (e: unknown) => {
        if (live) loadError = errorMessage(e);
      },
    );
    return () => {
      live = false;
    };
  });

  // The commit to go back to, and what that changes: null while the task at that time is being read.
  let target = $state<Log | null>(null);
  let changes = $state<RestoreChange[] | null>(null);
  let busy = $state(false);
  let error = $state("");
  const when = (l: Log) => formatDateTime(l.date, new Date());

  async function ask(l: Log) {
    target = l;
    changes = null;
    error = "";
    try {
      const then = await showAt(api, show.ulid, l.hash, show.store);
      if (target !== l) return;
      changes = diffForRestore(show, then, {
        idOf: (u) => {
          const t = cache.task(u);
          return t ? did.current(t.store, t.id) : u;
        },
        propLabel: (n) => sc.current?.props[n]?.label || n,
      });
    } catch (e) {
      if (target === l) {
        error = errorMessage(e);
        changes = [];
      }
    }
  }

  // Closing while the restore runs would leave its error in a dialog nobody sees.
  function close() {
    if (!busy) target = null;
  }

  async function restore() {
    if (!target || busy) return;
    busy = true;
    error = "";
    try {
      const r = await api.restoreTaskAt(show.ulid, target.hash, { store: show.store });
      cache.applyWrite({ task: r.task, commit: r.commit });
      toast({
        message:
          r.note || `${did.current(show.store, show.id)} を ${when(target)} の状態に戻しました`,
      });
      target = null;
    } catch (e) {
      error = errorMessage(e);
    } finally {
      busy = false;
    }
  }
</script>

<section aria-labelledby="hist-h">
  <div class="head">
    <h2 id="hist-h">履歴</h2>
    <span class="hint">git log から · 1 操作 1 コミット</span>
  </div>
  {#if loadError}
    <p class="error" role="alert">{loadError}</p>
  {/if}
  <ol>
    {#each logs as l, i (l.hash)}
      <li>
        <span class="time">{when(l)}</span>
        <div class="main">
          <p>{l.author.name} · {opLabel(l.op)} <span class="subject">{l.subject}</span></p>
          {#if automationOf(l)}
            {@const a = automationOf(l)}
            <p class="auto">自動化 · {eventLabel(a?.event ?? "")} · ルール {a?.rule}</p>
          {/if}
          {#each rejectionsOf(l).filter((r) => r.task === show.ulid) as r (r.rule)}
            <p class="auto">自動化を拒否 · ルール {r.rule} · {r.reason}</p>
          {/each}
          <p class="trailer">Hedgehug-Op: {l.op ?? "-"} · {l.hash.slice(0, 7)}</p>
        </div>
        {#if i > 0 && !readonly}
          <Button size="sm" variant="ghost" onclick={() => void ask(l)}>この時点に戻す</Button>
        {/if}
      </li>
    {/each}
  </ol>
</section>

{#if target}
  <Dialog
    open
    title="{did.current(show.store, show.id)} を {when(target)} の状態に戻す"
    width={620}
    cli={cliRestoreAt(did.current(show.store, show.id), target.hash)}
    onclose={close}
  >
    {#if changes === null}
      <p class="muted">この時点との違いを調べています…</p>
    {:else if changes.length === 0 && !error}
      <p>この時点と同じです。戻す内容はありません。</p>
    {:else if changes.length > 0}
      <p class="muted">変わる項目だけを表示しています。戻す操作も 1 コミットとして履歴に残ります</p>
      <table>
        <thead>
          <tr>
            <th>項目</th>
            <th>今</th>
            <th>戻したあと</th>
          </tr>
        </thead>
        <tbody>
          {#each changes as c (c.field)}
            <tr>
              <th scope="row">{c.label}</th>
              <td>{c.from}</td>
              <td>{c.to}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
    {#snippet footer()}
      <Button disabled={busy} onclick={close}>キャンセル</Button>
      {#if changes && changes.length > 0}
        <Button variant="confirm" disabled={busy} onclick={() => void restore()}
          >{changes.length}
          項目を戻す</Button
        >
      {/if}
    {/snippet}
  </Dialog>
{/if}

<style>
  .head {
    display: flex;
    gap: var(--space-8);
    align-items: baseline;
    margin-bottom: var(--space-8);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .hint,
  .subject,
  .auto,
  .trailer,
  .muted {
    font-size: var(--fs-12);
    color: var(--muted);
  }
  ol {
    margin: 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: var(--space-10);
  }
  li {
    display: flex;
    gap: var(--space-12);
    align-items: flex-start;
  }
  .time {
    flex: none;
    width: 7.5em;
    font-family: var(--font-mono);
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .main {
    flex: 1;
    min-width: 0;
  }
  p {
    margin: 0;
    font-size: var(--fs-13);
  }
  .trailer {
    font-family: var(--font-mono);
    overflow-wrap: anywhere;
  }
  table {
    width: 100%;
    margin-top: var(--space-8);
    border-collapse: collapse;
    font-size: var(--fs-13);
  }
  th,
  td {
    padding: var(--space-6) var(--space-8);
    border-bottom: var(--hairline) solid var(--line-soft);
    text-align: left;
    overflow-wrap: anywhere;
  }
  thead th {
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .error {
    color: var(--danger);
    font-size: var(--fs-12);
  }
</style>
