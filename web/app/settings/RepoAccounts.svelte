<script lang="ts">
  import { combine } from "@atlaskit/pragmatic-drag-and-drop/combine";
  import {
    draggable,
    dropTargetForElements,
  } from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
  import { reorder } from "@atlaskit/pragmatic-drag-and-drop/reorder";
  import {
    attachClosestEdge,
    extractClosestEdge,
  } from "@atlaskit/pragmatic-drag-and-drop-hitbox/closest-edge";
  import { getReorderDestinationIndex } from "@atlaskit/pragmatic-drag-and-drop-hitbox/util/get-reorder-destination-index";
  import { onMount } from "svelte";
  import type { Attachment } from "svelte/attachments";
  import { type Schemas, unwrap } from "../../core/api/client";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import Button from "../ui/Button.svelte";

  // repo_accounts of config.yaml. The order is what the rules mean, so each change sends the whole
  // list. The buttons do what the drag does, for the keyboard.
  type Rule = Schemas["RepoAccountView"];

  const c = core();
  const accounts = useSubscribe(c.accounts, () => c.accounts.accounts());

  let rules = $state.raw<Rule[]>([]);
  let busy = $state(false);
  let adding = $state(false);
  let match = $state("");
  let account = $state("");

  onMount(() => {
    void unwrap(c.api.GET("/api/repo-accounts"))
      .then((r) => {
        rules = r.rules;
      })
      .catch((e: unknown) => toast({ message: errorMessage(e) }));
  });

  async function save(next: Rule[]): Promise<boolean> {
    busy = true;
    try {
      const r = await unwrap(c.api.PUT("/api/repo-accounts", { body: { rules: next } }));
      rules = r.rules;
      return true;
    } catch (e) {
      toast({ message: errorMessage(e) });
      return false;
    } finally {
      busy = false;
    }
  }

  const move = (from: number, to: number) =>
    save(reorder({ list: rules, startIndex: from, finishIndex: to }));

  async function add(e: SubmitEvent) {
    e.preventDefault();
    const typed = match.trim();
    if (typed === "" || account === "" || busy) return;
    if (await save([...rules, { match: typed, account }])) {
      adding = false;
      match = "";
      account = "";
    }
  }

  const dnd =
    (index: number): Attachment<HTMLElement> =>
    (element) =>
      combine(
        draggable({ element, getInitialData: () => ({ ruleIndex: index }) }),
        dropTargetForElements({
          element,
          canDrop: ({ source }) => typeof source.data.ruleIndex === "number",
          getData: ({ input }) =>
            attachClosestEdge(
              { ruleIndex: index },
              { element, input, allowedEdges: ["top", "bottom"] },
            ),
          onDrop: ({ source, self }) => {
            const from = source.data.ruleIndex as number;
            const to = getReorderDestinationIndex({
              startIndex: from,
              indexOfTarget: index,
              closestEdgeOfTarget: extractClosestEdge(self.data),
              axis: "vertical",
            });
            if (to !== from) void move(from, to);
          },
        }),
      );
</script>

<section class="block" aria-labelledby="repo-accounts-h">
  <div class="head">
    <h2 id="repo-accounts-h">GitHub 連携で使うアカウント</h2>
    <span class="msg">PR・Issue を取得するとき、リポジトリごとに上から順に当てはめます</span>
  </div>
  {#if rules.length > 0}
    <ol>
      {#each rules as r, i (i)}
        <li {@attach dnd(i)}>
          <span class="n">{i + 1}</span>
          <code>{r.match}</code>
          <span aria-hidden="true">→</span>
          <span class="account">{r.account}</span>
          <Button
            size="sm"
            variant="ghost"
            aria-label="上へ: {r.match}"
            disabled={busy || i === 0}
            onclick={() => void move(i, i - 1)}
          >
            ↑
          </Button>
          <Button
            size="sm"
            variant="ghost"
            aria-label="下へ: {r.match}"
            disabled={busy || i === rules.length - 1}
            onclick={() => void move(i, i + 1)}
          >
            ↓
          </Button>
          <Button
            size="sm"
            variant="ghost"
            aria-label="削除: {r.match}"
            disabled={busy}
            onclick={() => void save(rules.filter((_, at) => at !== i))}
          >
            削除
          </Button>
        </li>
      {/each}
    </ol>
  {:else}
    <p class="msg">ルールがなければ、Store のアカウントで読みます</p>
  {/if}
  {#if adding}
    <form class="add" onsubmit={add}>
      <label>
        <span>リポジトリ</span>
        <input class="mono" autocomplete="off" placeholder="github.com/org-a/*" bind:value={match}>
      </label>
      <label>
        <span>アカウント</span>
        <select bind:value={account}>
          <option value="" disabled>選んでください</option>
          {#each accounts.current as a (a.id)}
            <option value={a.name}>{a.name}</option>
          {/each}
        </select>
      </label>
      <Button
        variant="confirm"
        type="submit"
        disabled={busy || match.trim() === "" || account === ""}
      >
        追加
      </Button>
      <Button onclick={() => (adding = false)}>やめる</Button>
    </form>
  {:else}
    <div><Button size="sm" onclick={() => (adding = true)}>ルールを追加</Button></div>
  {/if}
</section>

<style>
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--space-10);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-14);
    font-weight: 600;
  }
  .msg {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-12);
  }
  ol {
    display: flex;
    flex-direction: column;
    margin: 0;
    padding: 0;
    border: 1px solid var(--line);
    border-radius: var(--radius-8);
    list-style: none;
  }
  li {
    display: flex;
    align-items: center;
    gap: var(--space-12);
    min-height: var(--row-h);
    padding: var(--space-4) var(--space-12);
    cursor: grab;
  }
  li + li {
    border-top: 1px solid var(--line);
  }
  .n {
    width: 1.25rem;
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-12);
  }
  code {
    flex: 1;
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
    overflow-wrap: anywhere;
  }
  .account {
    width: 7rem;
    font-weight: 500;
  }
  .add {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: var(--space-10);
  }
  .add label {
    display: flex;
    flex: 1 1 10rem;
    flex-direction: column;
    gap: var(--space-6);
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  .add input,
  .add select {
    min-height: var(--control-h);
    padding: 0 var(--space-8);
    border: var(--hairline) solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink);
    font: inherit;
    font-weight: 400;
  }
  .add .mono {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
</style>
