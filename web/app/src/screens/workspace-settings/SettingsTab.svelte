<script lang="ts">
  import { readOnlyBand, type SettingsTab, settingsTabs } from "@hedgehug/core";
  import { ReadOnlyBand, Tabs } from "@hedgehug/ui";
  import { workspaceSettings } from "../../wire";
  import GeneralPanel from "./GeneralPanel.svelte";

  // 設定タブの枠(03「範囲と画面の構成」)。サブタブはアドレスに入れず、履歴にも積まない。
  interface Props {
    storeId: string;
    workspaceId: string;
    storeName: string;
  }

  let { storeId, workspaceId, storeName }: Props = $props();

  const state = workspaceSettings.state;
  $effect(() => {
    void workspaceSettings.open(storeId, workspaceId);
  });
</script>

{#if $state && $state.storeId === storeId && $state.detail.id === workspaceId}
  <div class="subtabs">
    <Tabs
      tabs={settingsTabs}
      current={$state.tab}
      onselect={(id) => workspaceSettings.selectTab(id as SettingsTab)}
    />
  </div>
  <div class="body">
    {#if $state.readOnly}
      <ReadOnlyBand text={readOnlyBand(storeName)} />
    {/if}
    {#if $state.tab === "general"}
      <GeneralPanel state={$state} />
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
