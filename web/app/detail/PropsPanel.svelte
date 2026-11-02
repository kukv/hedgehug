<script lang="ts">
  import type { ShowView } from "../../core/api/client";
  import type { PatchField } from "../../core/save";
  import { canParent, parentRule } from "../../core/schema";
  import { core } from "../context";
  import { useSubscribe } from "../svelte-store";
  import Avatar from "../ui/Avatar.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import FieldInput from "./FieldInput.svelte";
  import type { Option } from "./field-types";
  import PropField from "./PropField.svelte";
  import { propRows, statusOptions } from "./prop-rows";

  // parentKind is the kind of the parent as the screen read it (the cache may not have the parent when
  // the detail is opened by its URL); undefined: not read yet.
  let {
    show,
    readonly = false,
    parentKind: readKind,
  }: { show: ShowView; readonly?: boolean; parentKind?: string } = $props();

  const { cache, save, schema, stores } = core();
  const sc = useSubscribe(schema, () => schema.get(show.store));
  // Whether a field has a save that the server has not answered; a fresh object on every change of the SaveController.
  const pend = useSubscribe(save, () => ({
    editing: (f: PatchField) => save.editing(show.ulid, f),
  }));
  $effect(() => {
    void schema.load(show.store);
  });

  // null: no parent; undefined: there is one whose kind is not known, so no kind is ruled out here (the server decides).
  const parentKind = $derived(
    show.parent ? (readKind ?? cache.task(show.parent)?.kind ?? undefined) : null,
  );
  const kindOptions = $derived<Option[]>(
    (sc.current?.kinds ?? []).map((k) => {
      const current = sc.current;
      const usable =
        k.name === show.kind ||
        !current ||
        parentKind === undefined ||
        canParent(current, k.name, parentKind);
      return {
        value: k.name,
        label: k.label || k.name,
        disabled: !usable,
        title: usable || !current ? undefined : parentRule(current, k.name),
      };
    }),
  );
  const rows = $derived(sc.current ? propRows(sc.current, show.props) : []);
  const showRepos = $derived(
    (show.repos ?? []).length > 0 ||
      sc.current?.kinds?.find((k) => k.name === show.kind)?.role === "container",
  );

  function edit(field: PatchField, value: unknown) {
    save.edit(show.ulid, show.store, field, value);
  }
  const commit = () => void save.flush(show.ulid);
  const text = (v: unknown) => (typeof v === "string" ? v : JSON.stringify(v));
</script>

<dl>
  <dt>Store</dt>
  <dd><StoreBadge store={show.store} tone={stores.tone(show.store)} /></dd>

  <dt>種類</dt>
  <dd>
    <FieldInput
      editor="select"
      label="種類"
      value={show.kind}
      options={kindOptions}
      nullable={false}
      {readonly}
      onedit={(v) => edit("kind", v)}
      oncommit={commit}
    />
  </dd>

  <dt>ステータス</dt>
  <dd>
    <FieldInput
      editor="select"
      label="ステータス"
      value={show.status}
      options={statusOptions(sc.current)}
      nullable={false}
      {readonly}
      onedit={(v) => edit("status", v)}
      oncommit={commit}
    />
  </dd>

  <dt>担当者</dt>
  <dd>
    <FieldInput
      editor="chips"
      label="担当者"
      value={show.assignees ?? []}
      {readonly}
      onedit={(v) => edit("assignees", v)}
      oncommit={commit}
    />
  </dd>

  <dt>ラベル</dt>
  <dd>
    <FieldInput
      editor="chips"
      label="ラベル"
      value={show.labels ?? []}
      {readonly}
      onedit={(v) => edit("labels", v)}
      oncommit={commit}
    />
  </dd>

  {#each rows as r (r.name)}
    {#if r.editor === null}
      <dt class="undef">{r.label}（未定義）</dt>
      <dd class="undef">{text(r.value)}</dd>
    {:else}
      <dt>{r.label}</dt>
      <dd>
        <PropField
          row={r}
          value={r.value}
          {readonly}
          pending={pend.current.editing(`props.${r.name}`)}
          onedit={(v) => edit(`props.${r.name}`, v)}
          oncommit={commit}
        />
      </dd>
    {/if}
  {/each}

  {#if showRepos}
    <dt>リポジトリ</dt>
    <dd>
      <FieldInput
        editor="chips"
        label="リポジトリ"
        value={show.repos ?? []}
        {readonly}
        onedit={(v) => edit("repos", v)}
        oncommit={commit}
      />
    </dd>
  {/if}
</dl>

<section class="watchers">
  <h3>ウォッチャー</h3>
  <ul aria-label="ウォッチャー">
    {#each show.watchers ?? [] as w (w)}
      <li><Avatar name={w} /><span>{w}</span></li>
    {/each}
  </ul>
  <p class="hint">作成者・担当者・メンションで自動追加</p>
</section>

<style>
  dl {
    display: grid;
    grid-template-columns: 7.5em minmax(0, 1fr);
    gap: var(--space-6) var(--space-8);
    align-items: center;
    margin: 0;
  }
  dt {
    font-size: var(--fs-12);
    color: var(--muted);
  }
  dd {
    margin: 0;
    min-width: 0;
    font-size: var(--fs-13);
  }
  .undef {
    font-style: italic;
    color: var(--ink-2);
  }
  .watchers {
    margin-top: var(--space-16);
  }
  h3 {
    margin: 0 0 var(--space-6);
    font-size: var(--fs-12);
    font-weight: 600;
    color: var(--muted);
  }
  ul {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-6);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    font-size: var(--fs-12);
  }
  .hint {
    margin: var(--space-6) 0 0;
    font-size: var(--fs-12);
    color: var(--muted);
  }
</style>
