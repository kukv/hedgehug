<script lang="ts">
  import { type ShowView, type TaskView, unwrap } from "../../core/api/client";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import StatusIcon from "../icons/StatusIcon.svelte";
  import { onLinkClick, router } from "../router";
  import { useSubscribe } from "../svelte-store";
  import Button from "../ui/Button.svelte";
  import FieldInput from "./FieldInput.svelte";
  import { createLookup } from "./lookup";
  import {
    addRefBody,
    buildGroups,
    type RefPatch,
    type RelGroup,
    type RelItem,
    relationKinds,
    removeRefBody,
  } from "./relation-groups";
  import TaskPicker from "./TaskPicker.svelte";

  let { show, readonly = false }: { show: ShowView; readonly?: boolean } = $props();

  const { api, cache, schema } = core();
  const sc = useSubscribe(schema, () => schema.get(show.store));

  let groups = $state<RelGroup[]>([]);
  $effect(() => {
    const cur = show;
    const def = sc.current;
    if (!def) return;
    const lookup = createLookup({ api, cache, store: cur.store });
    let live = true;
    void buildGroups(cur, def, lookup).then((g) => {
      if (live) groups = g;
    });
    return () => {
      live = false;
    };
  });

  let adding = $state(false);
  let kind = $state("blocked_by");
  let error = $state("");
  const kinds = $derived(sc.current ? relationKinds(sc.current) : []);
  const taken = $derived(groups.find((g) => g.key === kind)?.items.map((i) => i.ulid) ?? []);
  const categoryOf = (status: string | null) =>
    sc.current?.statuses?.find((s) => s.name === status)?.category ?? "todo";

  async function patch(body: RefPatch): Promise<boolean> {
    error = "";
    try {
      const r = await unwrap(
        api.PATCH("/api/tasks/{id}", {
          params: { path: { id: show.ulid }, query: { store: show.store } },
          body,
        }),
      );
      cache.applyWrite({ task: r.task, commit: r.commit });
      return true;
    } catch (e) {
      error = errorMessage(e);
      return false;
    }
  }

  async function add(t: TaskView) {
    if (!sc.current) return;
    if (await patch(addRefBody(show, sc.current, kind, t.ulid))) adding = false;
  }

  function remove(g: RelGroup, item: RelItem) {
    if (sc.current) void patch(removeRefBody(show, sc.current, g.key, item.raw));
  }
</script>

<section aria-labelledby="rel-h">
  <div class="head">
    <h2 id="rel-h">関連</h2>
    <span class="hint">同じ Store（{show.store}）のタスクだけを参照できます</span>
    {#if !readonly}
      <Button size="sm" onclick={() => (adding = !adding)}>関連を追加</Button>
    {/if}
  </div>

  {#if adding && !readonly}
    <div class="add">
      <FieldInput
        editor="select"
        label="関連の種類"
        value={kind}
        options={kinds.map((k) => ({ value: k.key, label: k.label }))}
        nullable={false}
        onedit={(v) => (kind = String(v))}
        oncommit={() => {}}
      />
      <TaskPicker store={show.store} exclude={[show.ulid, ...taken]} onpick={add} />
    </div>
  {/if}
  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#each groups as g (g.key)}
    <fieldset class="group" aria-label={g.label}>
      <h3>{g.label} <span class="note">{g.note}</span></h3>
      <ul>
        {#each g.items as it (it.raw)}
          <li>
            <StatusIcon status={it.status ?? "todo"} category={categoryOf(it.status)} />
            <a
              class="id"
              href={router.href({ name: "task", ulid: it.ulid })}
              onclick={(e) => onLinkClick(e, router.href({ name: "task", ulid: it.ulid }))}
              >{it.id}</a
            >
            <span class="title">{it.title}</span>
            {#if g.removable && !readonly}
              <button
                type="button"
                class="remove"
                aria-label="{it.id} を外す"
                onclick={() => remove(g, it)}
              >
                ×
              </button>
            {/if}
          </li>
        {/each}
      </ul>
    </fieldset>
  {:else}
    <p class="empty">関連はありません</p>
  {/each}
</section>

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
  .note,
  .empty {
    font-size: var(--fs-12);
    color: var(--muted);
  }
  .head .hint {
    flex: 1;
  }
  .add {
    display: grid;
    grid-template-columns: 12em minmax(0, 1fr);
    gap: var(--space-8);
    margin-bottom: var(--space-8);
  }
  .group {
    margin: var(--space-12) 0 0;
    padding: 0;
    border: 0;
    min-width: 0;
  }
  h3 {
    margin: 0 0 var(--space-4);
    font-size: var(--fs-12);
    font-weight: 600;
    color: var(--ink-2);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    display: flex;
    gap: var(--space-8);
    align-items: center;
    min-height: var(--control-h-sm);
    font-size: var(--fs-13);
  }
  .id {
    font-family: var(--font-mono);
    font-size: var(--fs-12);
    color: var(--muted);
    text-decoration: none;
  }
  .title {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .remove {
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
  }
  .error {
    margin: var(--space-6) 0;
    font-size: var(--fs-12);
    color: var(--danger);
  }
</style>
