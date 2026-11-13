<script lang="ts">
  import type { DetailView, PropertyInput, Route } from "@hedgehug/core";
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
          project={v.item.projectId}
          settings={v.settings}
          disabled={v.readOnly}
          onchange={(value, at) => void change(row.property.id, value, at)}
          ontoggle={(id, on) => void detail.toggleValue(row.property.id, id, on)}
        />
      </dd>
    {/each}
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
