<script lang="ts">
  import type { RelationCandidate } from "@hedgehug/core";

  // 関連の相手の候補(02「関連」の「追加」)。選べない候補は無効にして、理由を添える。
  interface Props {
    candidates: RelationCandidate[];
    onchoose: (candidate: RelationCandidate) => void;
  }

  let { candidates, onchoose }: Props = $props();
</script>

{#if candidates.length > 0}
  <ul class="cands" aria-label="候補">
    {#each candidates as c (c.id)}
      <li>
        <button
          type="button"
          class="cand"
          disabled={c.disabledReason !== ""}
          onclick={() => onchoose(c)}
        >
          <code class="id">{c.display}</code>
          <span class="cand-title">{c.title}</span>
          {#if c.workspaceName}
            <span class="muted">{c.workspaceName}</span>
          {/if}
          {#if c.disabledReason}
            <span class="reason">{c.disabledReason}</span>
          {/if}
        </button>
      </li>
    {/each}
  </ul>
{/if}

<style>
  .cands {
    list-style: none;
    margin: 0;
    padding: 0;
    border: 1px solid var(--line);
    border-radius: 7px;
  }
  .cand {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 5px 8px;
    border: 0;
    background: none;
    text-align: left;
    cursor: pointer;
  }
  .cand:hover:not(:disabled) {
    background: var(--hover);
  }
  .cand:disabled {
    cursor: not-allowed;
    color: var(--muted);
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .cand-title {
    flex: 1;
    min-width: 0;
  }
  .muted {
    color: var(--muted);
    font-size: 12.5px;
  }
  .reason {
    font-size: 11.5px;
    color: var(--danger);
  }
</style>
