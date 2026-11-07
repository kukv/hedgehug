<script lang="ts">
  import { cliSync } from "../../core/cli";
  import { formatDuration } from "../../core/format";
  import { core } from "../context";
  import SyncCard from "../sidebar/SyncCard.svelte";
  import { useSubscribe } from "../svelte-store";
  import CliLine from "../ui/CliLine.svelte";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import DoctorSection from "./DoctorSection.svelte";
  import PurgeSection from "./PurgeSection.svelte";

  // A window that is hidden fetches every max(sync.interval, 10 minutes) (internal/app/server/loop.go).
  const HIDDEN_FETCH = 10 * 60;

  const { conn, stores } = core();
  const statuses = useSubscribe(conn, () => [...conn.sync.values()]);
  const dir = useSubscribe(stores, () => stores.stores().map((s) => s.name));

  // sync.interval and sync.push_interval are of config.yaml, not of a Store: the first one speaks for all.
  const timing = $derived(statuses.current[0] ?? null);
  const many = $derived(dir.current.length > 1);
</script>

<div class="page">
  <header class="head">
    <h1>同期と保守</h1>
  </header>
  <p class="lead">このマシンだけの設定です（Hedgehug の設定ファイル config.yaml の sync）</p>

  <section class="sec" aria-labelledby="timing-h">
    <h2 id="timing-h">同期のタイミング</h2>
    {#if timing}
      <dl class="rows">
        <div>
          <dt>push</dt>
          <dd>書き込みの {formatDuration(timing.push_interval)}後にまとめて push します</dd>
        </div>
        <div>
          <dt>fetch</dt>
          <dd>
            この画面を表示している間は
            {formatDuration(timing.interval)}ごと、別のタブやウィンドウに隠れている間は
            {formatDuration(Math.max(timing.interval, HIDDEN_FETCH))}ごとに fetch します
          </dd>
        </div>
      </dl>
      <p class="note">
        変えるには、config.yaml の <span class="mono">sync.interval</span>・<span class="mono"
          >sync.push_interval</span
        >
        を編集して <span class="mono">hh ui</span> を起動し直してください。
      </p>
    {:else}
      <p class="note">間隔は、同期の状態を受け取ってから表示します</p>
    {/if}
  </section>

  <section class="sec" aria-labelledby="state-h">
    <h2 id="state-h">同期の状態</h2>
    {#if statuses.current.length === 0}
      <p class="note">同期の状態をまだ受け取っていません</p>
    {:else}
      {#each statuses.current as st (st.store)}
        <section class="store" aria-labelledby="store-h-{st.store}">
          <h3 id="store-h-{st.store}" class="sr">Store: {st.store}</h3>
          {#if many}
            <StoreBadge store={st.store} tone={stores.tone(st.store)} />
          {/if}
          <SyncCard store={st.store} />
          <CliLine command={cliSync(many ? st.store : undefined)} />
        </section>
      {/each}
    {/if}
  </section>

  <section class="sec" aria-labelledby="doctor-h">
    <h2 id="doctor-h">診断と修復</h2>
    {#each dir.current as name (name)}
      <DoctorSection store={name} {many} />
    {/each}
  </section>

  <section class="sec" aria-labelledby="purge-h">
    <h2 id="purge-h">削除済みタスクの完全削除</h2>
    <PurgeSection stores={dir.current} />
  </section>
</div>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
    max-width: 48rem;
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--space-12);
  }
  h1 {
    margin: 0;
    font-size: var(--fs-19);
    font-weight: 600;
  }
  h2 {
    margin: 0;
    font-size: var(--fs-15);
    font-weight: 600;
  }
  .lead {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-13);
  }
  .sec {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }
  .rows {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    margin: 0;
  }
  .rows div {
    display: flex;
    gap: var(--space-12);
    font-size: var(--fs-13);
  }
  dt {
    flex: 0 0 4rem;
    color: var(--label);
  }
  dd {
    margin: 0;
  }
  .note {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
    line-height: 1.6;
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
  .mono {
    font-family: var(--font-mono);
  }
  .store {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-8);
    max-width: 24rem;
  }
  .store > :global(.card) {
    align-self: stretch;
  }
</style>
