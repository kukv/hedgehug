<script lang="ts">
  import { KINDS } from "../../core/kinds";
  import type { Template } from "../../core/planning";
  import type { SchemaView } from "../../core/schema";
  import type { ScopedDraft } from "../../core/scoped";
  import { core } from "../context";

  type Props = {
    store: string;
    schema: SchemaView;
    draft: ScopedDraft;
    busy: boolean;
    onchange: (d: ScopedDraft) => void;
  };
  let { store, schema, draft, busy, onchange }: Props = $props();

  const { api } = core();
  const uid = $props.id();

  let all = $state.raw<Template[]>([]);
  $effect(() => {
    const s = store;
    let live = true;
    api
      .listTemplates({ store: s })
      .then((r) => {
        if (live) all = r ?? [];
      })
      .catch(() => undefined);
    return () => {
      live = false;
    };
  });

  const toggle = (name: string, on: boolean) =>
    onchange({
      ...draft,
      templates: on ? [...draft.templates, name] : draft.templates.filter((t) => t !== name),
    });
  function setDefault(kind: string, name: string) {
    const { [kind]: _, ...rest } = draft.templateDefaults;
    onchange({ ...draft, templateDefaults: name === "" ? rest : { ...rest, [kind]: name } });
  }
</script>

<section class="block" aria-labelledby="{uid}-h">
  <h2 id="{uid}-h">テンプレート</h2>
  <p class="msg">
    使うテンプレートを絞ります。1 つも選ばなければ Store のテンプレートをすべて使えます
  </p>
  <fieldset aria-labelledby="{uid}-h">
    {#each all as t (t.name)}
      <label>
        <input
          type="checkbox"
          checked={draft.templates.includes(t.name)}
          disabled={busy}
          onchange={(e) => toggle(t.name, e.currentTarget.checked)}
        >
        {t.name}
      </label>
    {:else}
      <span class="msg">テンプレートはありません</span>
    {/each}
  </fieldset>
  {#if all.length > 0}
    <div class="defaults">
      {#each KINDS as k (k.name)}
        <label>
          <span>{k.label} の既定</span>
          <select
            value={draft.templateDefaults[k.name] ?? ""}
            disabled={busy}
            onchange={(e) => setDefault(k.name, e.currentTarget.value)}
          >
            <option value="">
              {schema.templates?.default?.[k.name]
                ? `Store の既定（${schema.templates.default[k.name]}）`
                : "なし"}
            </option>
            {#each all as t (t.name)}
              <option value={t.name}>{t.name}</option>
            {/each}
          </select>
        </label>
      {/each}
    </div>
  {/if}
</section>

<style>
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-10);
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
  }
  fieldset {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-12);
    margin: 0;
    padding: 0;
    border: 0;
  }
  fieldset label {
    display: inline-flex;
    gap: var(--space-6);
    align-items: center;
  }
  .defaults {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    max-width: 420px;
  }
  .defaults label {
    display: flex;
    gap: var(--space-12);
    align-items: center;
    justify-content: space-between;
  }
  select {
    min-height: 30px;
    border: 1px solid var(--line-input);
    border-radius: var(--radius-6);
    background: var(--surface);
  }
</style>
