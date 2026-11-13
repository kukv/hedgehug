<script lang="ts">
  import {
    type AutomationRule,
    type RuleProject,
    type RuleSection,
    routeOfProject,
    rulesProjects,
    rulesProjectsLabel,
    type SettingsState,
  } from "@hedgehug/core";
  import { get } from "svelte/store";
  import { catalog, navigation } from "../../wire";

  // サブタブの上の 1 行(03「範囲と画面の構成」)。名前を押すと、そのプロジェクトの設定タブの該当の区画を開く。
  interface Props {
    state: SettingsState;
    section: RuleSection;
    // 削除済みでないプロジェクト
    projects: RuleProject[];
    rules?: AutomationRule[];
  }

  let { state: s, section, projects, rules = [] }: Props = $props();

  const line = $derived(rulesProjects(s.detail.settings, section, projects, rules));

  function open(id: string) {
    const route = routeOfProject(get(catalog.stores), s.storeId, id, "settings");
    navigation.go(route, `rules-${section}`);
  }
</script>

{#if line}
  <p class="line">
    {rulesProjectsLabel}
    {#each line.projects as p, i (p.id)}
      {#if i > 0}
        ・
      {/if}
      <button type="button" class="link" onclick={() => open(p.id)}>{p.display} {p.title}</button>
    {/each}
    ({line.projects.length}
    件)
  </p>
{/if}

<style>
  .line {
    margin: 0 0 12px;
    font-size: 12.5px;
    color: var(--muted);
  }
  .link {
    all: unset;
    cursor: pointer;
    text-decoration: underline;
  }
  .link:focus-visible {
    outline: 2px solid currentColor;
  }
</style>
