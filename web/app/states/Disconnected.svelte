<script lang="ts">
  import Icon from "../icons/Icon.svelte";

  let { lastOpenAt }: { lastOpenAt: number | null } = $props();

  const pad = (n: number) => String(n).padStart(2, "0");
  const clock = $derived.by(() => {
    if (lastOpenAt === null) return null;
    const d = new Date(lastOpenAt);
    return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
  });
</script>

<div class="veil" role="alert">
  <Icon name="wifi-off" size={26} />
  <b>Hedgehug のサーバーに接続できません</b>
  <span class="text">
    端末で hh ui を実行してください。
    {#if clock}
      表示中の内容は {clock} 時点のもので、編集はできません。
    {/if}
  </span>
  <span class="retry">
    <span class="spin"><Icon name="spinner" size={12} /></span>
    5 秒ごとに再接続しています
  </span>
</div>

<style>
  .veil {
    position: fixed;
    inset: 0;
    z-index: 150;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-10);
    padding: var(--space-20);
    background: color-mix(in srgb, var(--surface) 82%, transparent);
    color: var(--ink-2);
    text-align: center;
  }
  b {
    color: var(--ink);
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .text {
    line-height: 1.6;
  }
  .retry {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .spin {
    display: inline-flex;
    color: var(--accent);
    animation: spin 1.2s linear infinite;
  }
  @media (prefers-reduced-motion: reduce) {
    .spin {
      animation: none;
    }
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
