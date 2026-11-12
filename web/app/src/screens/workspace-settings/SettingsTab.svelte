<script lang="ts">
  import { readOnlyBand, type SettingsTab, settingsTabs } from "@hedgehug/core";
  import { ReadOnlyBand, Tabs } from "@hedgehug/ui";
  import { workspaceSettings } from "../../wire";
  import GeneralPanel from "./GeneralPanel.svelte";
  import PropertiesPanel from "./PropertiesPanel.svelte";
  import ValuesPanel from "./ValuesPanel.svelte";

  // 設定タブの枠(03「範囲と画面の構成」)。サブタブはアドレスに入れず、履歴にも積まない。
  interface Props {
    storeId: string;
    workspaceId: string;
    storeName: string;
  }

  let { storeId, workspaceId, storeName }: Props = $props();

  const settings = workspaceSettings.state;
  $effect(() => {
    void workspaceSettings.open(storeId, workspaceId);
  });
</script>

{#if $settings && $settings.storeId === storeId && $settings.detail.id === workspaceId}
  <div class="subtabs">
    <Tabs
      tabs={settingsTabs}
      current={$settings.tab}
      onselect={(id) => workspaceSettings.selectTab(id as SettingsTab)}
    />
  </div>
  <div class="body">
    {#if $settings.readOnly}
      <ReadOnlyBand text={readOnlyBand(storeName)} />
    {/if}
    {#if $settings.tab === "general"}
      <GeneralPanel state={$settings} />
    {:else if $settings.tab === "properties"}
      <PropertiesPanel state={$settings} />
    {:else if $settings.tab === "statuses"}
      <ValuesPanel state={$settings} list="statuses" />
    {:else if $settings.tab === "priorities"}
      <ValuesPanel state={$settings} list="priorities" />
    {:else if $settings.tab === "systems"}
      <ValuesPanel state={$settings} list="systems" />
    {/if}
  </div>
{/if}

<style>
  /* 親の本文の余白の中に入るので、タブの帯は左右の余白を打ち消して端まで伸ばす。 */
  .subtabs {
    margin: -16px -24px 0;
  }
  .body {
    padding: 16px 24px 40px;
  }
</style>
