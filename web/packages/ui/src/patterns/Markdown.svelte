<script lang="ts">
  import type { MarkdownBlock, MarkdownInline, MarkdownListItem } from "../types";

  // Markdown の表示(02「Markdown」)。{@html} は使わない。文字はすべて本文として出すので、タグや
  // javascript: のような文字列は、ただの文字になる。リンクは http(s) だけ(core の字句分けが決める)。
  interface Props {
    blocks: MarkdownBlock[];
  }

  let { blocks }: Props = $props();
</script>

{#snippet inlines(list: MarkdownInline[])}
  {#each list as n, i (i)}
    {#if n.kind === "bold"}
      <strong>{n.text}</strong>
    {:else if n.kind === "code"}
      <code>{n.text}</code>
    {:else if n.kind === "link"}
      <a href={n.url} target="_blank" rel="noopener noreferrer">{n.url}</a>
    {:else if n.kind === "mention"}
      <span class="mention" data-mention={n.login.toLowerCase()}>@{n.login}</span>
    {:else}
      {n.text}
    {/if}
  {/each}
{/snippet}

{#snippet items(list: MarkdownListItem[])}
  {#each list as it, i (i)}
    <li class:marked={it.check !== "none"}>
      {#if it.check === "checked"}
        <span class="mark" role="img" aria-label="チェック済み">☑</span>
      {:else if it.check === "unchecked"}
        <span class="mark" role="img" aria-label="未チェック">☐</span>
      {/if}
      {@render inlines(it.inlines)}
    </li>
  {/each}
{/snippet}

<div class="md">
  {#each blocks as b, i (i)}
    {#if b.kind === "heading"}
      <svelte:element this={`h${b.level + 3}`} class="h{b.level}">
        {@render inlines(b.inlines)}
      </svelte:element>
    {:else if b.kind === "list"}
      {#if b.ordered}
        <ol>
          {@render items(b.items)}
        </ol>
      {:else}
        <ul>
          {@render items(b.items)}
        </ul>
      {/if}
    {:else if b.kind === "code"}
      <pre><code>{b.text}</code></pre>
    {:else}
      <p>{@render inlines(b.inlines)}</p>
    {/if}
  {/each}
</div>

<style>
  .md {
    display: flex;
    flex-direction: column;
    gap: 6px;
    overflow-wrap: anywhere;
  }
  p,
  h4,
  h5,
  h6 {
    margin: 0;
  }
  .h1 {
    font-size: 16px;
  }
  .h2 {
    font-size: 14.5px;
  }
  .h3 {
    font-size: 13.5px;
  }
  ul,
  ol {
    margin: 0;
    padding-left: 20px;
  }
  li.marked {
    list-style: none;
    margin-left: -20px;
  }
  .mark {
    color: var(--muted);
  }
  pre {
    margin: 0;
    padding: 8px 10px;
    background: var(--sunken);
    border-radius: 6px;
    overflow-x: auto;
  }
  code {
    background: var(--sunken);
    border-radius: 4px;
    padding: 1px 4px;
  }
  pre code {
    background: none;
    padding: 0;
  }
  .mention {
    color: var(--accent-ink);
    background: var(--accent-weak);
    border-radius: 4px;
    padding: 0 3px;
  }
</style>
