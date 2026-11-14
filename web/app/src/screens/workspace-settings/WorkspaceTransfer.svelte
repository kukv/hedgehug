<script lang="ts">
  import {
    type SettingsState,
    workspaceTargetLabel,
    workspaceTransferButton,
    workspaceTransferNote,
    workspaceTransferTitle,
  } from "@hedgehug/core";
  import { Button, Select } from "@hedgehug/ui";
  import { untrack } from "svelte";
  import { workspaceTransfer } from "../../wireActions";

  // 「一般」の「別の Store へ移動」の区画(03「別の Store へ移動」)。候補・選んだ移動先・閲覧のみの無効は
  // core の workspaceTransfer が持つ。「移動…」は選んだ移動先の予告を読んで確認ダイアログを出す。
  interface Props {
    state: SettingsState;
  }

  let { state: s }: Props = $props();

  const section = workspaceTransfer.section;
  // 出しているワークスペースの区画だけを出す(別のワークスペースの読み込みの結果を出さない)。
  const sec = $derived(
    $section?.workspace.storeId === s.storeId && $section.workspace.id === s.detail.id
      ? $section
      : null,
  );

  // 候補は、別のワークスペースになったときと、閲覧のみかが変わったときだけ読み直す
  // (s は読み直しのたびに新しいオブジェクトになるので、値で比べる $derived を見る)。
  const key = $derived(`${s.storeId}/${s.detail.id}`);
  const readOnly = $derived(s.readOnly);
  $effect(() => {
    void key;
    void readOnly;
    untrack(
      () =>
        void workspaceTransfer.load({
          storeId: s.storeId,
          id: s.detail.id,
          name: s.detail.name,
          prefix: s.detail.prefix,
        }),
    );
  });
</script>

<div class="field">
  <h2>{workspaceTransferTitle}</h2>
  <p class="text">{workspaceTransferNote}</p>
  {#if sec}
    <div class="row">
      {#if sec.empty}
        <p class="text muted">{sec.empty}</p>
      {:else}
        <Select
          label={workspaceTargetLabel}
          size="sm"
          options={sec.options}
          value={sec.value}
          disabled={sec.disabled}
          title={sec.note}
          onchange={(v) => workspaceTransfer.choose(v)}
        />
      {/if}
      <Button
        size="sm"
        disabled={!sec.canOpen}
        title={sec.note}
        onclick={() => void workspaceTransfer.open()}
        >{workspaceTransferButton}</Button
      >
    </div>
  {/if}
</div>

<style>
  .field {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  h2 {
    font-size: 14px;
    margin: 0;
  }
  .text {
    margin: 0;
  }
  .muted {
    color: var(--muted);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
  }
</style>
