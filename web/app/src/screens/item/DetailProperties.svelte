<script lang="ts">
  import {
    chosen,
    type DetailView,
    type PropertyInput,
    type Route,
    readOnlyNote,
    releaseChoices,
    showRelease,
    showSprint,
    sprintChoices,
  } from "@hedgehug/core";
  import { Select } from "@hedgehug/ui";
  import { detail, links } from "../../wire";
  import PropertyField from "./PropertyField.svelte";

  // 詳細パネルのプロパティの区画(02「上から順の構成」、決定 19)。保存できなかったら、入力を保存済みの値に戻す。
  interface Props {
    v: DetailView;
  }

  let { v }: Props = $props();
  let version = $state(0);
  // ワークスペースの設定タブのアドレス(/s/{store}/w/{prefix}/settings)。
  const settingsRoute = $derived<Route>({
    name: "workspace",
    store: v.store.name,
    prefix: v.workspace.prefix,
    tab: "settings",
  });

  // スプリント・リリースの選択肢(02「スプリント・リリース」)。使えない参照はバックログ・「なし」と出す(F1-5)。
  const sprints = $derived(sprintChoices(v.plan, v.item.sprint));
  const releases = $derived(releaseChoices(v.plan, v.item.release));
  const options = (cs: { id: string; label: string }[]) =>
    cs.map((c) => ({ value: c.id, label: c.label }));
  // assign は、スプリント・リリースを保存する。保存できなかったら、セレクトを保存済みの値に戻す。
  async function assign(save: Promise<boolean>) {
    if (!(await save)) version += 1;
  }

  // at は、入力欄にフォーカスしたときのアイテム。開いているアイテムが変わっていたら、保存せずに入力を戻す。
  async function change(propertyId: string, value: PropertyInput, at = v.item.id) {
    if (at !== v.item.id || !(await detail.setProperty(at, propertyId, value))) version += 1;
  }
</script>

<section class="peek-sec" aria-label="プロパティ">
  <dl class="props">
    {#each v.rows as row (`${v.item.id}-${row.property.id}-${version}`)}
      <dt>
        {row.property.name}
        {#if row.extra}
          <span class="extra">追加</span>
        {/if}
      </dt>
      <dd>
        <PropertyField
          {row}
          itemId={v.item.id}
          settings={v.settings}
          disabled={v.readOnly}
          onchange={(value, at) => void change(row.property.id, value, at)}
          ontoggle={(id, on) => void detail.toggleValue(row.property.id, id, on)}
        />
      </dd>
    {/each}
    {#key `${v.item.id}-${version}`}
      {#if showSprint(v.item.kind)}
        <dt>スプリント</dt>
        <dd>
          <Select
            label="スプリント"
            options={options(sprints)}
            value={chosen(sprints, v.item.sprint)}
            disabled={v.readOnly}
            title={v.readOnly ? readOnlyNote : undefined}
            onchange={(id) => void assign(detail.setSprint(id))}
          />
        </dd>
      {/if}
      {#if showRelease(v.item.kind)}
        <dt>リリース</dt>
        <dd>
          <Select
            label="リリース"
            options={options(releases)}
            value={chosen(releases, v.item.release)}
            disabled={v.readOnly}
            title={v.readOnly ? readOnlyNote : undefined}
            onchange={(id) => void assign(detail.setRelease(id))}
          />
        </dd>
      {/if}
    {/key}
  </dl>
  <a
    class="props-link"
    href={links.href(settingsRoute)}
    onclick={(e) => links.click(e, settingsRoute)}
    onauxclick={(e) => links.click(e, settingsRoute)}
    >プロパティを追加・変更</a
  >
</section>

<style>
  .props {
    display: grid;
    grid-template-columns: 96px minmax(0, 1fr);
    gap: 6px 8px;
    align-items: center;
    margin: 0 0 8px;
    font-size: 12.5px;
  }
  .props dt {
    color: var(--muted);
  }
  .props dd {
    margin: 0;
    min-width: 0;
  }
  .extra {
    font-size: 10.5px;
    color: var(--muted);
    opacity: 0.8;
  }
  .props-link {
    font-size: 12px;
  }
</style>
