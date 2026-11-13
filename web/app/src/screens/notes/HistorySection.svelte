<script lang="ts">
  import { type NoteTarget, relativeTime } from "@hedgehug/core";
  import { Button } from "@hedgehug/ui";
  import { onDestroy, untrack } from "svelte";
  import { itemHistory, projectHistory, syncNow } from "../../wire";

  // 履歴(02「履歴」)。アイテムの詳細パネルとプロジェクトの概要で同じ。新しい順に 40 件まで出し、
  // それより古いものがあるときは「すべて表示」を置く。1 行に、操作した人・変更の文・時刻を出す。
  // refresh は、画面の読み直し(保存・反映)のたびに新しくなる値(アイテムなら v.item)。履歴も読み直す。
  // target.kind は、この区画を作ったあとで変えない(詳細パネルは item、概要は project)。
  interface Props {
    target: NoteTarget;
    refresh?: unknown;
  }

  let { target, refresh }: Props = $props();
  const list = untrack(() => (target.kind === "item" ? itemHistory : projectHistory));
  const model = list.state;

  $effect(() => {
    void refresh;
    void list.load({ kind: target.kind, storeId: target.storeId, id: target.id });
  });
  onDestroy(() => list.reset());

  const when = (at: string) => relativeTime(new Date(at), $syncNow);
  const exact = (at: string) => new Date(at).toLocaleString("ja-JP");
</script>

<section class="peek-sec" aria-label="履歴">
  <h3>履歴</h3>
  {#if $model.loaded && $model.rows.length === 0}
    <p class="muted">履歴はありません</p>
  {/if}
  {#if $model.rows.length > 0}
    <ol class="rows">
      {#each $model.rows as r, i (i)}
        <li>
          <span class="who">{r.who}</span>
          <span class="text">{r.text}</span>
          <time datetime={r.at} title={exact(r.at)}>{when(r.at)}</time>
        </li>
      {/each}
    </ol>
  {/if}
  {#if $model.more}
    <div>
      <Button size="sm" onclick={() => list.showAll()}>すべて表示</Button>
    </div>
  {/if}
</section>

<style>
  .peek-sec {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  h3 {
    font-size: 13px;
    margin: 0;
  }
  .muted {
    color: var(--muted);
    font-size: 12.5px;
    margin: 0;
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  li {
    display: flex;
    gap: 8px;
    align-items: baseline;
    padding: 4px 0;
    border-bottom: 1px solid var(--line);
    font-size: 12.5px;
  }
  .who {
    flex: none;
    color: var(--muted);
  }
  .text {
    flex: 1;
    min-width: 0;
  }
  time {
    flex: none;
    color: var(--muted);
    font-size: 12px;
    white-space: nowrap;
  }
</style>
