<script lang="ts">
  import type { Schemas } from "../../core/api/client";
  import { formatRelative } from "../../core/format";
  import { errorMessage } from "../../core/messages";
  import { readOnlyReason, type StoreTone } from "../../core/stores";
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";
  import StoreColorPicker from "./StoreColorPicker.svelte";
  import StoreDot from "./StoreDot.svelte";

  // One mounted Store: its colour, whether it can be written, where it syncs, and the check that
  // shows or hides it. The details hold what the Store is made of and the operations on it.
  // lastVisible is true when this is the only Store that is shown: its check stays on.
  type Props = {
    store: Schemas.StoreItem;
    lastVisible: boolean;
    onremove: () => void;
  };
  let { store, lastVisible, onremove }: Props = $props();

  const c = core();
  const view = useSubscribe(c.stores, () => ({
    readOnly: c.stores.readOnly(store.name),
    tone: c.stores.tone(store.name),
    failure: c.stores.failure(store.name),
  }));
  const failure = $derived(view.current.failure ? errorMessage(view.current.failure) : null);

  const readOnly = $derived(view.current.readOnly);
  // The saved colour, else the colour of its place; grey while it cannot be written.
  const dot = $derived<StoreTone>(
    readOnly ? "readonly" : ((store.color || view.current.tone) as StoreTone),
  );
  const remotes = $derived(store.remotes ?? []);
  // The connection's sync state changes while the page is open; a coarse clock moves the relative time on.
  const sync = useSubscribe(c.conn, () => c.conn.sync.get(store.name)?.last_success);
  let now = $state(new Date());
  $effect(() => {
    const t = setInterval(() => (now = new Date()), 30_000);
    return () => clearInterval(t);
  });
  const lastSuccess = $derived(sync.current ?? store.last_success);
  const synced = $derived.by(() => {
    if (!lastSuccess) return "未同期";
    const when = formatRelative(lastSuccess, now);
    return when === "今" ? "たった今同期" : `${when}に同期`;
  });
  const cannotHide = $derived(lastVisible && !store.hidden);

  let open = $state(false);
  let colorButton: HTMLElement | null = $state(null);
  let picking = $state(false);
  let busy = $state(false);
  const uid = $props.id();

  // A failed write says why in a toast; the card keeps what the server has.
  async function write(run: () => Promise<unknown>): Promise<boolean> {
    if (busy) return false;
    busy = true;
    try {
      await run();
      // The write is done; a failed reload must not read as a failed write (the SSE resync catches up).
      await c.stores.reload().catch(() => undefined);
      return true;
    } catch (e) {
      toast({ message: errorMessage(e) });
      return false;
    } finally {
      busy = false;
    }
  }

  const patch = (body: { color?: string; hidden?: boolean }) =>
    write(() => c.api.updateStore(store.name, body));

  function pick(color: string): void {
    picking = false;
    void patch({ color });
  }

  async function makeDefault(): Promise<void> {
    const done = await write(() => c.api.setDefaultStore({ name: store.name }));
    if (done) toast({ message: `既定の Store を ${store.name} にしました` });
  }
</script>

<article class="card" aria-label={store.name}>
  <header class="top">
    <button
      type="button"
      class="color"
      bind:this={colorButton}
      aria-label="{store.name} の色を変える（このマシンだけ）"
      aria-haspopup="menu"
      aria-expanded={picking}
      title="色を変える（このマシンだけ）"
      onclick={() => {
        picking = !picking;
      }}
    >
      <StoreDot tone={dot} />
    </button>
    <h3>{store.name}</h3>
    {#if readOnly}
      <span class="badge" data-kind="readonly"><Icon name="forbid" size={14} />閲覧のみ</span>
      <span class="why">{readOnlyReason(readOnly)}</span>
    {:else}
      <span class="badge" data-kind="write"><Icon name="check" size={14} />書き込み可</span>
    {/if}
    {#if store.default_store}
      <span class="badge" data-kind="default">既定の Store</span>
    {/if}
    <span class="spacer"></span>
    <label class="shown">
      <input
        type="checkbox"
        checked={!store.hidden}
        disabled={cannotHide || busy}
        aria-describedby={cannotHide ? `${uid}-why` : undefined}
        onchange={(e) => {
          if (cannotHide) return;
          // The box shows what the server has: the list is read again after the write.
          const next = e.currentTarget.checked;
          e.currentTarget.checked = !store.hidden;
          void patch({ hidden: !next });
        }}
      >
      表示
    </label>
    <button
      type="button"
      class="toggle"
      aria-label={open ? `閉じる: ${store.name}` : `詳細を開く: ${store.name}`}
      aria-expanded={open}
      onclick={() => {
        open = !open;
      }}
    >
      <Icon name={open ? "chevron-down" : "chevron-right"} size={16} />
    </button>
  </header>

  <p class="line">
    <span class="mono">{remotes[0] ?? "remote なし"}</span>
    <span class="synced">{synced}</span>
  </p>
  {#if store.account}
    <p class="line mono">{store.account}</p>
  {/if}
  {#if failure}
    <p class="fail" role="alert">{failure}</p>
  {/if}
  {#if cannotHide}
    <p id="{uid}-why" class="note">この Store は非表示にできません</p>
  {/if}

  {#if open}
    <dl class="details">
      <div class="row">
        <dt>remote</dt>
        <dd>
          {#if remotes.length === 0}
            なし
          {:else}
            <ul>
              {#each remotes as r, i (r)}
                <li>
                  <span class="mono">{r}</span>
                  {#if i > 0}
                    <span class="mirror">ミラー</span>
                  {/if}
                </li>
              {/each}
            </ul>
          {/if}
        </dd>
      </div>
      <div class="row">
        <dt>アカウント</dt>
        <dd>{store.account ?? "なし"}</dd>
      </div>
      <div class="row">
        <dt>ローカルのパス</dt>
        <dd class="mono">{store.path}</dd>
      </div>
    </dl>
    <div class="actions">
      {#if !store.default_store}
        <Button disabled={busy} onclick={makeDefault}>既定にする</Button>
      {/if}
      <Button variant="danger" onclick={onremove}>マウントを解除</Button>
    </div>
  {/if}
</article>

{#if picking}
  <StoreColorPicker
    anchor={colorButton}
    current={store.color ?? ""}
    label="{store.name} の色"
    onselect={pick}
    onclose={() => {
      picking = false;
    }}
  />
{/if}

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding: var(--space-12) var(--space-16);
    border: 1px solid var(--line);
    border-radius: var(--radius-12);
    background: var(--surface);
  }
  .top {
    display: flex;
    align-items: center;
    gap: var(--space-8);
  }
  .spacer {
    flex: 1;
  }
  h3 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .color,
  .toggle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--space-28);
    height: var(--space-28);
    border: 0;
    border-radius: var(--radius-6);
    background: transparent;
    color: var(--ink-2);
    cursor: pointer;
  }
  .color:hover,
  .toggle:hover {
    background: var(--hover);
  }
  .badge {
    display: inline-flex;
    align-items: center;
    gap: var(--space-4);
    padding: var(--space-4) var(--space-8);
    border-radius: var(--radius-pill);
    background: var(--sunken);
    color: var(--ink-2);
    font-size: var(--fs-12);
  }
  .badge[data-kind="write"] {
    background: var(--success-weak);
    color: var(--success-ink);
  }
  .badge[data-kind="readonly"] {
    background: var(--warning-weak);
    color: var(--warning);
  }
  .badge[data-kind="default"] {
    background: var(--accent-weak);
    color: var(--accent-ink);
  }
  .fail {
    margin: 0;
    color: var(--danger-ink);
    font-size: var(--fs-12);
  }
  .why {
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .shown {
    display: inline-flex;
    align-items: center;
    gap: var(--space-6);
    font-size: var(--fs-13);
  }
  .line {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-12);
    margin: 0;
    padding-left: calc(var(--space-28) + var(--space-8));
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
    overflow-wrap: anywhere;
  }
  .note {
    margin: 0;
    padding-left: calc(var(--space-28) + var(--space-8));
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .details {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    margin: 0;
    padding: var(--space-8) 0 0 calc(var(--space-28) + var(--space-8));
    border-top: 1px solid var(--line);
    font-size: var(--fs-12);
  }
  .row {
    display: flex;
    gap: var(--space-12);
  }
  dt {
    flex: 0 0 8rem;
    color: var(--muted);
  }
  dd {
    margin: 0;
    min-width: 0;
  }
  ul {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .mirror {
    margin-left: var(--space-8);
    padding: 0 var(--space-6);
    border-radius: var(--radius-pill);
    background: var(--sunken);
    color: var(--ink-2);
  }
  .actions {
    display: flex;
    gap: var(--space-8);
    padding-left: calc(var(--space-28) + var(--space-8));
  }
</style>
