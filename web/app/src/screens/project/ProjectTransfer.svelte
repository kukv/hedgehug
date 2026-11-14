<script lang="ts">
  import {
    type ProjectView,
    projectTargetLabel,
    projectTransferNote,
    projectTransferOk,
  } from "@hedgehug/core";
  import { Button, Select } from "@hedgehug/ui";
  import { untrack } from "svelte";
  import { projectTransfer } from "../../wireActions";

  // 設定タブの「別のワークスペースへ移動」の区画(02「名前の変更・移動・削除」の移動)。候補・選んだ移動先・
  // 閲覧のみの無効は core の projectTransfer が持つ。「移動」は選んだ移動先の予告を読んで確認ダイアログを出す。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  const p = $derived(view.project);
  const section = projectTransfer.section;
  // 出しているプロジェクトの区画だけを出す(別のプロジェクトの読み込みの結果を出さない)。
  const s = $derived(
    $section?.project.storeId === p.storeId && $section.project.id === p.id ? $section : null,
  );

  // 候補は、別のプロジェクトになったときと、ワークスペースが変わったとき(同じ Store の中で移すと内部 ID も
  // 画面も同じまま、いまのワークスペースが候補から外れる)と、閲覧のみかが変わったときだけ読み直す
  // (p は読み直しのたびに新しいオブジェクトになるので、値で比べる $derived を見る)。
  const projectKey = $derived(`${p.storeId}/${p.id}/${p.workspaceId}`);
  const readOnly = $derived(view.store.readOnly);
  $effect(() => {
    void projectKey;
    void readOnly;
    untrack(() => void projectTransfer.load(p));
  });
</script>

<section class="sec">
  <h2>別のワークスペースへ移動</h2>
  <p class="text">{projectTransferNote}</p>
  {#if s}
    <div class="row">
      {#if s.empty}
        <p class="text muted">{s.empty}</p>
      {:else}
        <Select
          label={projectTargetLabel}
          size="sm"
          options={s.options}
          value={s.value}
          disabled={s.disabled}
          title={s.note}
          onchange={(v) => projectTransfer.choose(v)}
        />
      {/if}
      <Button
        size="sm"
        disabled={!s.canOpen}
        title={s.note}
        onclick={() => void projectTransfer.open()}
        >{projectTransferOk}</Button
      >
    </div>
  {/if}
</section>

<style>
  .sec {
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
