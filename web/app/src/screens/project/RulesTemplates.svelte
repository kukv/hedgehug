<script lang="ts">
  import {
    hasRules,
    type Kind,
    type ProjectView,
    projectTemplateView,
    readOnlyNote,
    rulesOf,
  } from "@hedgehug/core";
  import { Checkbox, KindIcon, type KindName, Select } from "@hedgehug/ui";
  import { projectsPort, rulesEditor } from "../../projectEditor";
  import ResetRulesButton from "./ResetRulesButton.svelte";

  // 「テンプレート」の区画(03「プロジェクトのテンプレート」)。上に使うテンプレート、下に種類ごとの既定。
  // 操作のたびに保存し、断られたら選び直す前の値に戻る(読み直しで画面が保存済みの値になる)。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  const disabled = $derived(view.store.readOnly);
  const title = $derived(disabled ? readOnlyNote : undefined);
  const data = $derived(projectTemplateView(view.settings, view.project.id));
  const empty = $derived(!hasRules(rulesOf(view.settings, view.project.id), "templates"));

  function use(templateId: string, used: boolean) {
    const p = view.project;
    void rulesEditor.run(() => projectsPort.setTemplateUsed(p.storeId, p.id, templateId, used));
  }

  function setDefault(kind: Kind, value: string) {
    const p = view.project;
    void rulesEditor.run(() => projectsPort.setTemplateDefault(p.storeId, p.id, kind, value));
  }
</script>

<section class="sec">
  <h2 id="rules-templates" tabindex="-1">テンプレート</h2>
  <p class="help">使うテンプレート</p>
  <ul class="rows">
    {#each data.uses as u (u.id)}
      <li>
        <KindIcon kind={u.kind as KindName} />
        <Checkbox
          label={u.name}
          {disabled}
          {title}
          checked={u.checked}
          onchange={(v) => use(u.id, v)}
        />
      </li>
    {/each}
  </ul>
  {#if data.allUsable}
    <p class="help">すべて使えます</p>
  {/if}
  <p class="help">種類ごとの既定</p>
  <div class="defaults">
    {#each data.defaults as d (d.kind)}
      <div class="item">
        <span class="kind"><KindIcon kind={d.kind as KindName} />{d.kind}</span>
        <Select
          label={d.kind}
          options={d.options}
          {disabled}
          {title}
          bind:value={() => d.value, (v) => setDefault(d.kind, v)}
        />
      </div>
    {/each}
  </div>
  <ResetRulesButton {view} section="templates" label="テンプレート" {empty} />
</section>

<style>
  .sec {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  h2 {
    font-size: 14px;
    margin: 0;
  }
  .help {
    margin: 0;
    font-size: 12.5px;
    color: var(--muted);
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  li,
  .item,
  .kind {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .defaults {
    display: flex;
    flex-wrap: wrap;
    gap: 12px 24px;
  }
  .kind {
    gap: 6px;
    font-size: 12.5px;
    color: var(--ink-2);
  }
</style>
