<script lang="ts">
  import { searchHelp } from "@hedgehug/core";
  import { Dialog } from "@hedgehug/ui";

  // 検索ヘルプ(04「検索ヘルプ」)。閉じるボタンは Dialog の見出しの「閉じる」。
  interface Props {
    open: boolean;
    onclose: () => void;
  }

  let { open, onclose }: Props = $props();

  // `…` で囲んだところをコードの書体にする(奇数番目の区切りが中身)。
  const parts = (text: string) => text.split("`");
</script>

{#snippet rich(text: string)}
  {#each parts(text) as part, i (i)}
    {#if i % 2 === 1}
      <code>{part}</code>
    {:else}
      {part}
    {/if}
  {/each}
{/snippet}

<Dialog {open} title="検索ヘルプ" wide {onclose}>
  <p class="lead">{@render rich(searchHelp.lead)}</p>
  <table class="help">
    <thead>
      <tr>
        <th scope="col">書き方</th>
        <th scope="col">説明</th>
      </tr>
    </thead>
    <tbody>
      {#each searchHelp.rows as [syntax, text], i (i)}
        <tr>
          <td class="syntax">{@render rich(syntax)}</td>
          <td>{@render rich(text)}</td>
        </tr>
      {/each}
    </tbody>
  </table>
</Dialog>

<style>
  .lead {
    margin: 0 0 12px;
  }
  .help {
    width: 100%;
    border-collapse: collapse;
    font-size: 12.5px;
  }
  th,
  td {
    padding: 6px 8px;
    border-bottom: 1px solid var(--line);
    text-align: left;
    vertical-align: top;
  }
  th {
    color: var(--muted);
    font-weight: 600;
  }
  .syntax {
    white-space: nowrap;
  }
  code {
    font-family: var(--font-mono);
    font-size: 12px;
  }
</style>
