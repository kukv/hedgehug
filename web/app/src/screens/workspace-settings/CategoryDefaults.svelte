<script lang="ts">
  import { type Category, categoryLabels, readOnlyNote, type SettingsState } from "@hedgehug/core";
  import { Select } from "@hedgehug/ui";
  import { workspaceSettings } from "../../wire";

  // 分類ごとの既定のステータス(03「分類ごとの既定のステータス」)。
  interface Props {
    state: SettingsState;
  }

  let { state: s }: Props = $props();

  const categories = Object.keys(categoryLabels) as Category[];
  const optionsOf = (category: Category) =>
    s.detail.settings.statuses
      .filter((x) => x.category === category)
      .map((x) => ({ value: x.id, label: x.name }));

  function choose(category: Category, id: string) {
    if (id === s.detail.settings.categoryDefaults[category]) return;
    void workspaceSettings.run((p, st, w) => p.setCategoryDefault(st, w, category, id));
  }
</script>

<div class="defaults">
  {#each categories as category (category)}
    <div class="field">
      <span>{categoryLabels[category]}の既定</span>
      <Select
        label="{categoryLabels[category]}の既定"
        bind:value={
          () => s.detail.settings.categoryDefaults[category] ?? "",
          (id) => choose(category, id)
        }
        options={optionsOf(category)}
        disabled={s.readOnly}
        title={s.readOnly ? readOnlyNote : undefined}
      />
    </div>
  {/each}
</div>

<style>
  .defaults {
    display: flex;
    flex-wrap: wrap;
    gap: 12px 16px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 12.5px;
    color: var(--muted);
  }
</style>
