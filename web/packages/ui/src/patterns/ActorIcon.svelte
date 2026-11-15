<script lang="ts">
  // アクティビティの実行者のアイコン。人は Avatar、自動化は「A」、GitHub と同期は印、実行者なしは「hh」。
  import Avatar from "./Avatar.svelte";

  interface Props {
    kind: "user" | "automation" | "github" | "sync" | "none";
    name: string;
    title: string;
  }

  let { kind, name, title }: Props = $props();
</script>

{#if kind === "user"}
  <Avatar {name} {title} />
{:else}
  <span class="actor" role="img" {title} aria-label={title}>
    {#if kind === "automation"}
      A
    {:else if kind === "none"}
      hh
    {:else if kind === "github"}
      <svg
        data-glyph="github"
        width="12"
        height="12"
        viewBox="0 0 16 16"
        fill="currentColor"
        aria-hidden="true"
      >
        <path
          d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8z"
        />
      </svg>
    {:else}
      <svg
        data-glyph="sync"
        width="12"
        height="12"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2.5"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path d="M4 9h14l-4-4M20 15H6l4 4" />
      </svg>
    {/if}
  </span>
{/if}

<style>
  .actor {
    display: inline-grid;
    place-items: center;
    width: 18px;
    height: 18px;
    border-radius: 5px;
    flex: none;
    font-size: 10.5px;
    font-weight: 600;
    font-family: var(--font-mono);
    color: var(--surface);
    background: var(--muted);
  }
</style>
