<script lang="ts">
  import type { Rejected } from "../../core/github";
  import { core } from "../context";
  import { createLookup } from "../detail/lookup";
  import { onLinkClick, router } from "../router";
  import { useDisplayId } from "../svelte-store";

  // A rule that was refused on a commit that read many tasks: the commit names the task by its
  // ULID, so its ID is looked up; a task that cannot be read stays as its ULID.
  let { rejected, store }: { rejected: Rejected; store: string } = $props();

  const { api, cache, stores } = core();
  const did = useDisplayId(stores);
  const href = $derived(router.href({ name: "task", ulid: rejected.task }));
  let id = $state("");
  $effect(() => {
    let live = true;
    void createLookup({ api, cache, store })(rejected.task).then((c) => {
      if (live && c) id = did.current(store, c.id);
    });
    return () => {
      live = false;
    };
  });
</script>

<span class="rejected">
  自動化を拒否 · ルール {rejected.rule}
  <a {href} onclick={(e) => onLinkClick(e, href)}>{id || rejected.task}</a>
  {rejected.reason}
</span>

<style>
  .rejected {
    color: var(--danger-ink);
    font-size: var(--fs-12);
    overflow-wrap: anywhere;
  }
  a {
    font-family: var(--font-mono);
    font-size: var(--fs-mono);
  }
</style>
