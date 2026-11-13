<script lang="ts">
  import {
    type RuleProject,
    type RuleSection,
    readOnlyBand,
    type WorkspaceSettingsTab,
    workspaceSettingsTabs,
  } from "@hedgehug/core";
  import { ReadOnlyBand, Tabs } from "@hedgehug/ui";
  import { catalog, workspaceSettings } from "../../wire";
  import AutomationPanel from "./AutomationPanel.svelte";
  import GeneralPanel from "./GeneralPanel.svelte";
  import PropertiesPanel from "./PropertiesPanel.svelte";
  import RulesProjectsLine from "./RulesProjectsLine.svelte";
  import TemplatesPanel from "./TemplatesPanel.svelte";
  import ValuesPanel from "./ValuesPanel.svelte";
  import WorkflowPanel from "./WorkflowPanel.svelte";

  // 設定タブの枠(03「範囲と画面の構成」)。サブタブはアドレスに入れず、履歴にも積まない。
  interface Props {
    storeId: string;
    workspaceId: string;
    storeName: string;
  }

  let { storeId, workspaceId, storeName }: Props = $props();

  const settings = workspaceSettings.state;
  const stores = catalog.stores;
  // 削除済みでないプロジェクト(カタログにあるもの)。削除済みは出さず数えない(B5a-15)。
  const projects = $derived<RuleProject[]>(
    (
      $stores.find((x) => x.id === storeId)?.workspaces.find((w) => w.id === workspaceId)
        ?.projects ?? []
    ).map((p) => ({ id: p.id, display: p.displayId, title: p.title })),
  );
  const ruleSections: Partial<Record<WorkspaceSettingsTab, RuleSection>> = {
    statuses: "statuses",
    workflow: "workflow",
    templates: "templates",
    automation: "automation",
  };
  const section = $derived($settings ? ruleSections[$settings.tab] : undefined);
  // 親が Store の読み直しで作り直した値を渡しても、ID が同じなら開き直さない(入力の途中を消さないため)。
  const sid = $derived(storeId);
  const wid = $derived(workspaceId);
  $effect(() => {
    void workspaceSettings.open(sid, wid);
  });
</script>

{#if $settings && $settings.storeId === storeId && $settings.detail.id === workspaceId}
  <div class="subtabs">
    <Tabs
      tabs={workspaceSettingsTabs}
      current={$settings.tab}
      onselect={(id) => workspaceSettings.selectTab(id as WorkspaceSettingsTab)}
    />
  </div>
  <div class="body">
    {#if $settings.readOnly}
      <ReadOnlyBand text={readOnlyBand(storeName)} />
    {/if}
    {#if section}
      <RulesProjectsLine
        state={$settings}
        {section}
        {projects}
        rules={$settings.automation?.rules}
      />
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
    {:else if $settings.tab === "workflow"}
      <WorkflowPanel state={$settings} />
    {:else if $settings.tab === "templates"}
      <TemplatesPanel state={$settings} {projects} />
    {:else if $settings.tab === "automation"}
      <AutomationPanel state={$settings} {projects} />
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
