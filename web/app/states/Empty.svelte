<script lang="ts">
  import Icon from "../icons/Icon.svelte";
  import { kbdLabel } from "../keys";
  import Button from "../ui/Button.svelte";
  import CliLine from "../ui/CliLine.svelte";

  type Props = {
    variant: "empty" | "nomatch" | "noworkspace" | "notfound";
    scope?: "project" | "workspace" | "store" | "view";
    name?: string;
    note?: string;
    onCreate?: () => void;
    onClear?: () => void;
    onHome?: () => void;
    commands?: string[];
  };
  let {
    variant,
    scope = "project",
    name = "",
    note,
    onCreate,
    onClear,
    onHome,
    commands = [],
  }: Props = $props();

  const notfoundLabel = $derived(
    scope === "project"
      ? "プロジェクト"
      : scope === "store"
        ? `Store「${name}」`
        : scope === "view"
          ? `ビュー「${name}」`
          : `ワークスペース「${name}」`,
  );
</script>

{#if variant === "empty"}
  <div class="empty">
    <Icon name="folder" size={28} />
    <b>{name} にはまだタスクがありません</b>
    <span class="text">
      {scope === "project" ? "このプロジェクト" : "このワークスペース"}の中で
      <code>hh add</code>
      しても、ここに入ります。
    </span>
    {#if onCreate}
      <Button variant="primary" onclick={onCreate}>
        タスクを作成 <kbd>{kbdLabel("c")}</kbd>
      </Button>
    {/if}
  </div>
{:else if variant === "notfound"}
  <div class="empty">
    <Icon name="folder" size={28} />
    <b>{notfoundLabel}が見つかりません</b>
    {#if onHome}
      <Button size="sm" onclick={onHome}>ホームへ戻る</Button>
    {/if}
  </div>
{:else if variant === "noworkspace"}
  <div class="empty">
    <Icon name="folder" size={28} />
    <b>ワークスペースがありません</b>
    <span class="text">端末でワークスペースを足すと、ここにツリーが出ます。</span>
    {#each commands as c (c)}
      <CliLine command={c} />
    {/each}
  </div>
{:else}
  <div class="empty">
    <b>一致するタスクはありません</b>
    {#if note}
      <span class="text small">{note}</span>
    {/if}
    {#if onClear}
      <Button size="sm" onclick={onClear}>フィルタを解除</Button>
    {/if}
  </div>
{/if}

<style>
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-10);
    padding: 32px var(--space-20);
    border-radius: var(--radius-12);
    color: var(--muted-2);
    text-align: center;
  }
  b {
    color: var(--ink);
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .text {
    color: var(--ink-2);
    line-height: 1.6;
  }
  .small {
    font-size: var(--fs-12);
  }
  code {
    font-size: var(--fs-mono);
  }
  kbd {
    color: var(--accent-on);
    font-size: var(--fs-11);
  }
</style>
