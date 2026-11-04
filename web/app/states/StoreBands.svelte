<script lang="ts">
  import { unwrap } from "../../core/api/client";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import { readOnlySentence, useReadOnly } from "../read-only";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";

  // The bands above the page of a Store: it is read-only (and why), and it is hidden (with the way back).
  // spaced: the page has no gap of its own, so the bands keep their distance from what follows.
  let { store, spaced = false }: { store: string; spaced?: boolean } = $props();

  const c = core();
  const reason = useReadOnly(c.stores, () => store);
  const hidden = useSubscribe(c.stores, () => c.stores.hidden(store));
  let busy = $state(false);

  // A failed write says why in a toast; the band stays as it was.
  async function show(): Promise<void> {
    if (busy) return;
    busy = true;
    try {
      await unwrap(
        c.api.PATCH("/api/stores/{name}", {
          params: { path: { name: store } },
          body: { hidden: false },
        }),
      );
      // The write is done; a failed reload must not read as a failed write (the SSE resync catches up).
      await c.stores.reload().catch(() => undefined);
      toast({ message: `${store} を表示しました` });
    } catch (e) {
      toast({ message: errorMessage(e) });
    } finally {
      busy = false;
    }
  }
</script>

{#if reason.current !== null || hidden.current}
  <div class="bands" class:spaced>
    {#if reason.current !== null}
      <div class="band" role="status" aria-label="閲覧のみ">
        <Icon name="lock" size={16} />
        <span>{readOnlySentence(reason.current)}</span>
      </div>
    {/if}
    {#if hidden.current}
      <div class="band" role="status" aria-label="非表示">
        <Icon name="eye-off" size={16} />
        <span>この Store は非表示です</span>
        <button type="button" class="show" disabled={busy} aria-busy={busy} onclick={show}>
          表示する
        </button>
      </div>
    {/if}
  </div>
{/if}

<style>
  .bands {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }
  .spaced {
    margin-bottom: var(--space-12);
  }
  .band {
    display: flex;
    align-items: center;
    gap: var(--space-10);
    padding: var(--space-10) 14px;
    border-radius: var(--radius-10);
    line-height: 1.55;
    background: var(--ink);
    color: var(--surface);
  }
  .band span {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .band :global(svg) {
    color: var(--alert-on-dark);
  }
  .show {
    flex: none;
    min-height: var(--space-28);
    padding: var(--space-4) var(--space-12);
    border: 0;
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink);
    font: inherit;
    cursor: pointer;
  }
  .show:focus-visible {
    outline: calc(var(--hairline) * 2) solid var(--surface);
    outline-offset: var(--space-4);
  }
  .show:disabled {
    opacity: 0.6;
    cursor: default;
  }
</style>
