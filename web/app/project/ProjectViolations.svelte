<script lang="ts">
  import { untrack } from "svelte";
  import type { TaskView } from "../../core/api/client";
  import { errorMessage } from "../../core/messages";
  import { violationQuery } from "../../core/scoped";
  import { core } from "../context";
  import { onLinkClick, router } from "../router";
  import FormError from "../ui/FormError.svelte";
  import IdLabel from "../ui/IdLabel.svelte";

  // The tasks of the project that break the schema now. They change with the tasks and with the
  // schema, so the list is read again at both.
  let { ulid, store }: { ulid: string; store: string } = $props();

  const c = core();
  const uid = $props.id();

  let violations = $state.raw<TaskView[] | null>(null);
  let failure = $state<string | null>(null);
  let seq = 0;

  async function load(s: string): Promise<void> {
    const mine = ++seq;
    try {
      const r = await c.api.listTasks({ q: violationQuery(ulid), store: s });
      if (mine !== seq) return;
      violations = r.tasks ?? [];
      failure = null;
    } catch (e) {
      // The list that was read stays; the failure is shown above it.
      if (mine === seq) failure = errorMessage(e);
    }
  }

  $effect(() => {
    const s = store;
    // The list of another Store is not this one's.
    violations = null;
    failure = null;
    untrack(() => void load(s));
    const offs = [
      c.conn.on("schema.changed", (e) => {
        if (e.store === s) void load(s);
      }),
      c.conn.on("tasks.changed", (e) => {
        if (e.store === s) void load(s);
      }),
      c.conn.on("resync", () => void load(s)),
    ];
    return () => {
      for (const off of offs) off();
      // An answer that is still on its way is not wanted any more.
      seq++;
    };
  });
</script>

<section class="block" aria-labelledby="{uid}-bad">
  <h2 id="{uid}-bad">いまの違反</h2>
  {#if failure}
    <FormError message={failure} />
  {/if}
  {#if violations === null}
    {#if !failure}
      <p class="msg">読み込み中…</p>
    {/if}
  {:else if violations.length === 0}
    <p class="msg">違反しているタスクはありません</p>
  {:else}
    <p class="msg">いまの違反 {violations.length} 件</p>
    <ul class="bad" aria-label="違反しているタスク">
      {#each violations as t (t.ulid)}
        {@const href = router.href({ name: "task", ulid: t.ulid })}
        <li>
          <IdLabel id={t.id} ulid={t.ulid} {store} {href} onopen={(e) => onLinkClick(e, href)} />
          <span class="why">{(t.problems ?? []).map((p) => p.message).join(" / ")}</span>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding: var(--space-20) 0;
    border-bottom: 1px solid var(--line);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .msg {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.7;
  }
  .bad {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .bad li {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--space-8);
    font-size: var(--fs-13);
  }
  .why {
    color: var(--muted);
  }
</style>
