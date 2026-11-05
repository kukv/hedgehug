<script lang="ts">
  import { type Schemas, type ShowView, unwrap } from "../../core/api/client";
  import { cliWorklog, Placeholder } from "../../core/cli";
  import { errorMessage } from "../../core/messages";
  import { worklogLabel } from "../../core/planning";
  import { core } from "../context";
  import { useDisplayId, useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";

  let { show, readonly = false }: { show: ShowView; readonly?: boolean } = $props();

  const { api, accounts, stores } = core();
  const uid = $props.id();
  const did = useDisplayId(stores);

  // As with comments, only the author may delete an entry; the accounts say who that is.
  void accounts.load();
  const mine = useSubscribe(accounts, () =>
    accounts.loaded() && accounts.error() === null
      ? new Set(accounts.accounts().map((a) => a.id.toLowerCase()))
      : new Set<string>(),
  );

  let logs = $state.raw<Schemas["WorklogView"][]>([]);
  let rev = $state(0);
  $effect(() => {
    const params = { path: { id: show.ulid }, query: { store: show.store } };
    void show.worklog?.count;
    void show.updated;
    void rev;
    let live = true;
    unwrap(api.GET("/api/tasks/{id}/worklogs", { params }))
      .then((r) => {
        if (live) logs = r ?? [];
      })
      .catch(() => undefined);
    return () => {
      live = false;
    };
  });

  let time = $state("");
  let note = $state("");
  let date = $state("");
  let busy = $state(false);
  let failure = $state<string | null>(null);

  const total = $derived(show.worklog?.total ?? "0m");
  const cli = $derived(
    cliWorklog({
      id: did.current(show.store, show.id),
      time: time.trim() === "" ? new Placeholder("時間") : time.trim(),
      note,
      date,
    }),
  );

  async function add() {
    if (busy || time.trim() === "") return;
    busy = true;
    failure = null;
    try {
      await unwrap(
        api.POST("/api/tasks/{id}/worklogs", {
          params: { path: { id: show.ulid }, query: { store: show.store } },
          body: {
            duration: time.trim(),
            ...(note.trim() ? { note: note.trim() } : {}),
            ...(date ? { date } : {}),
          },
        }),
      );
      time = "";
      note = "";
      date = "";
      rev++;
    } catch (e) {
      failure = errorMessage(e);
    } finally {
      busy = false;
    }
  }

  async function remove(log: string) {
    try {
      await unwrap(
        api.DELETE("/api/tasks/{id}/worklogs/{log}", {
          params: { path: { id: show.ulid, log }, query: { store: show.store } },
        }),
      );
      rev++;
    } catch (e) {
      toast({ message: errorMessage(e) });
    }
  }
</script>

<section aria-labelledby="{uid}-h">
  <div class="head">
    <h2 id="{uid}-h">作業時間</h2>
    <span class="sum">{worklogLabel(total, show.props.estimate)}</span>
  </div>
  {#if logs.length > 0}
    <ul>
      {#each logs as l (l.ulid)}
        <li>
          <span class="mono">{l.date}</span>
          <b>{l.duration}</b>
          <span class="who">{l.author}</span>
          <span class="note">{l.body ?? ""}</span>
          {#if !readonly && mine.current.has(l.author.toLowerCase())}
            <Button
              size="sm"
              aria-label="{l.date} の記録を削除"
              onclick={() => void remove(l.ulid)}
            >
              削除
            </Button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
  {#if !readonly}
    <form
      onsubmit={(e) => {
        e.preventDefault();
        void add();
      }}
    >
      <input
        type="text"
        aria-label="時間"
        placeholder="1h30m"
        autocomplete="off"
        value={time}
        oninput={(e) => (time = e.currentTarget.value)}
      >
      <input
        class="grow"
        type="text"
        aria-label="メモ"
        placeholder="メモ"
        autocomplete="off"
        value={note}
        oninput={(e) => (note = e.currentTarget.value)}
      >
      <input
        type="date"
        aria-label="日付"
        value={date}
        oninput={(e) => (date = e.currentTarget.value)}
      >
      <Button type="submit" size="sm" disabled={busy || time.trim() === ""}>記録</Button>
    </form>
    {#if failure}
      <p class="bad" role="alert">{failure}</p>
    {/if}
    <CliLine command={cli} disabled={time.trim() === ""} />
  {/if}
</section>

<style>
  section {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }
  .head {
    display: flex;
    gap: var(--space-10);
    align-items: baseline;
  }
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .sum {
    color: var(--ink-2);
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    display: flex;
    gap: var(--space-10);
    align-items: center;
    min-height: 34px;
    border-top: 1px solid var(--line);
  }
  .mono,
  .who {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .mono {
    font-family: var(--font-mono);
  }
  b {
    font-weight: 600;
  }
  .note {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  form {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
    align-items: center;
  }
  input {
    width: 7em;
    min-height: 32px;
    padding: 0 var(--space-8);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-6);
  }
  input[type="date"] {
    width: 10em;
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  input.grow {
    flex: 1 1 160px;
  }
  .bad {
    margin: 0;
    color: var(--danger);
  }
</style>
