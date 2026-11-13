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

  // 文言は「ラベル + 名前(・でつなぐ)+ (N 件)」の切れ目に空白を入れない。名前だけがボタン。
  const parts = $derived.by(() => {
    if (!line) return [];
    const names = line.projects.flatMap((p, i) => [
      ...(i > 0 ? [{ text: "・" }] : []),
      { id: p.id, text: `${p.display} ${p.title}` },
    ]);
    return [{ text: rulesProjectsLabel }, ...names, { text: `(${line.projects.length} 件)` }];
  });

  function open(id: string) {
    const route = routeOfProject(get(catalog.stores), s.storeId, id, "settings");
    navigation.go(route, `rules-${section}`);
  }
</script>

{#if line}
  <p class="line" data-testid="rules-projects-line">
    {#each parts as part, i (i)}
      {#if part.id}
        <button type="button" class="link" onclick={() => open(part.id)}>{part.text}</button>
      {:else}
        {part.text}
      {/if}
    {/each}
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
