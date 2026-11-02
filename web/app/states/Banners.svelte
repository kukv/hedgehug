<script lang="ts">
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import { online } from "../online";
  import { summarizeSync } from "../sidebar/sync-summary";
  import { useSubscribe } from "../svelte-store";

  const { conn } = core();
  const statuses = useSubscribe(conn, () => conn.sync);
  const summary = $derived(summarizeSync(statuses.current.values(), online.current));
</script>

{#if summary.kind === "offline" || summary.kind === "error"}
  <div class="bands">
    {#if summary.kind === "offline"}
      <div class="band dark" role="status">
        <Icon name="wifi-off" size={16} />
        <span>オフラインです。変更はこのマシンにコミットし、つながったら push します。</span>
      </div>
    {:else}
      {#each summary.errors as e (e.store)}
        <div class="band bad" role="alert">
          <Icon name="warning" size={16} />
          <span>{e.store} を push できません（{e.message}）。変更はこのマシンに残っています。</span>
        </div>
      {/each}
    {/if}
  </div>
{/if}

<style>
  .bands {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding: var(--space-12) var(--gutter) 0;
  }
  .band {
    display: flex;
    align-items: center;
    gap: var(--space-10);
    padding: var(--space-10) 14px;
    border-radius: var(--radius-10);
    line-height: 1.55;
  }
  .band span {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .dark {
    background: var(--ink);
    color: var(--surface);
  }
  .dark :global(svg) {
    color: var(--alert-on-dark);
  }
  .bad {
    background: var(--danger-weak);
    color: var(--ink);
  }
  .bad :global(svg) {
    color: var(--danger);
  }
</style>
