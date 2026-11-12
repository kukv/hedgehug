<script lang="ts">
  import { kindDefaults, readOnlyNote, type SettingsState } from "@hedgehug/core";
  import { KindIcon, type KindName, Select } from "@hedgehug/ui";
  import { workspaceSettings } from "../../wire";

  // 種類ごとの既定のテンプレート(03「テンプレート」)。
  interface Props {
    state: SettingsState;
  }

  let { state: s }: Props = $props();

  const defaults = $derived(kindDefaults(s.detail.settings));
  const title = $derived(s.readOnly ? readOnlyNote : undefined);

  function change(kind: string, value: string) {
    void workspaceSettings.run((p, st, w) => p.setTemplateDefault(st, w, kind, value));
  }
</script>

<div class="defaults">
  {#each defaults as d (d.kind)}
    <div class="item">
      <span class="kind"><KindIcon kind={d.kind as KindName} />{d.kind}</span>
      <Select
        label={`${d.kind} の既定`}
        options={d.options}
        disabled={s.readOnly}
        {title}
        bind:value={() => d.value, (v) => change(d.kind, v)}
      />
    </div>
  {/each}
</div>

<style>
  .defaults {
    display: flex;
    flex-wrap: wrap;
    gap: 12px 24px;
  }
  .item {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .kind {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 12.5px;
    color: var(--ink-2);
  }
</style>
