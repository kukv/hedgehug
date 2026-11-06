<script lang="ts">
  import { onMount } from "svelte";
  import { core } from "../context";
  import Icon from "../icons/Icon.svelte";
  import { hrefOf, onLinkClick, router, type SettingsPage } from "../router";
  import { useSubscribe } from "../svelte-store";

  let { page }: { page: SettingsPage } = $props();

  const { api, conn, stores } = core();
  const names = useSubscribe(stores, () => stores.stores().map((s) => s.name));
  // Every Store is listed, the hidden ones too; one that cannot be written says so.
  const options = useSubscribe(stores, () =>
    stores.stores().map((s) => ({ name: s.name, readOnly: stores.readOnly(s.name) !== null })),
  );
  let version = $state<string | null>(null);

  // The version is a footnote: when it cannot be read, the footnote is not shown.
  onMount(() => {
    let gone = false;
    api
      .getHealth()
      .then((h) => {
        if (!gone) version = h.version;
      })
      .catch(() => undefined);
    return () => {
      gone = true;
    };
  });

  // The page of this machine names no Store: the one picked here (the first until one is) is the
  // Store that the items of a Store lead to.
  let picked = $state<string | null>(null);
  // On a page of a Store the pick follows that Store; on the general page it keeps the last one.
  const pageStore = $derived("store" in page ? page.store : null);
  $effect(() => {
    if (pageStore !== null) picked = pageStore;
  });
  const store = $derived(
    pageStore ??
      (picked !== null && names.current.includes(picked) ? picked : (names.current[0] ?? "")),
  );
  const accountsHref = hrefOf({ name: "settings", page: "accounts" });
  const storesHref = hrefOf({ name: "settings", page: "stores" });
  const generalHref = hrefOf({ name: "settings", page: "general" });
  const workspacesHref = $derived(hrefOf({ name: "settings", page: "workspaces", store }));
  const schemaHref = $derived(hrefOf({ name: "settings", page: "schema", store, tab: "statuses" }));
  const automationsHref = $derived(hrefOf({ name: "settings", page: "automations", store }));
  const homeHref = hrefOf({ name: "home" });
  // The repositories of all Stores the GitHub App is not installed for.
  const appMissing = useSubscribe(conn, () =>
    [...conn.sync.values()].reduce((n, s) => n + (s.github?.app_missing ?? 0), 0),
  );

  function pick(next: string) {
    if ("store" in page) router.go(hrefOf({ name: "settings", ...page, store: next }));
    else picked = next;
  }
</script>

<nav class="nav" aria-label="設定のナビゲーション">
  <a class="back" href={homeHref} onclick={(e) => onLinkClick(e, homeHref)}>
    <span aria-hidden="true">←</span>
    Hedgehug に戻る
  </a>
  <h2>設定</h2>

  <section class="group" aria-label="このマシン">
    <span class="cap">このマシン</span>
    <a
      class="item"
      class:on={page.page === "accounts"}
      href={accountsHref}
      aria-current={page.page === "accounts" ? "page" : undefined}
      onclick={(e) => onLinkClick(e, accountsHref)}
    >
      <Icon name="people" size={14} />アカウントと認証
      {#if appMissing.current > 0}
        <span class="badge" title="GitHub App の警告 {appMissing.current} 件">
          {appMissing.current}
        </span>
      {/if}
    </a>
    <a
      class="item"
      class:on={page.page === "stores"}
      href={storesHref}
      aria-current={page.page === "stores" ? "page" : undefined}
      onclick={(e) => onLinkClick(e, storesHref)}
    >
      <Icon name="database" size={14} />マウント中の Store
    </a>
    <a
      class="item"
      class:on={page.page === "general"}
      href={generalHref}
      aria-current={page.page === "general" ? "page" : undefined}
      onclick={(e) => onLinkClick(e, generalHref)}
    >
      <Icon name="sync" size={14} />同期と保守
    </a>
  </section>

  <section class="group" aria-label="Store ごとの設定">
    <span class="cap">Store ごとの設定</span>
    {#if names.current.length >= 2}
      <label class="store">
        <span class="label">設定する Store</span>
        <select value={store} onchange={(e) => pick(e.currentTarget.value)}>
          {#each options.current as o (o.name)}
            <option value={o.name}>{o.readOnly ? `${o.name}（閲覧のみ）` : o.name}</option>
          {/each}
        </select>
      </label>
    {/if}
    {#if store !== ""}
      <a
        class="item"
        class:on={page.page === "workspaces"}
        href={workspacesHref}
        aria-current={page.page === "workspaces" ? "page" : undefined}
        onclick={(e) => onLinkClick(e, workspacesHref)}
      >
        <Icon name="workspace" size={14} />ワークスペース
      </a>
      <a
        class="item"
        class:on={page.page === "schema"}
        href={schemaHref}
        aria-current={page.page === "schema" ? "page" : undefined}
        onclick={(e) => onLinkClick(e, schemaHref)}
      >
        <Icon name="checklist" size={14} />スキーマ
      </a>
      <a
        class="item"
        class:on={page.page === "automations"}
        href={automationsHref}
        aria-current={page.page === "automations" ? "page" : undefined}
        onclick={(e) => onLinkClick(e, automationsHref)}
      >
        <Icon name="sync" size={14} />自動化と GitHub
      </a>
    {/if}
    <p class="note">
      Store ごとの設定は、その Store を使う全員に共有されます（git に書かれます）。
    </p>
  </section>

  {#if version}
    <p class="foot">Hedgehug {version}</p>
  {/if}
</nav>

<style>
  .nav {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
    min-height: 100%;
    padding: 14px var(--space-10);
  }
  .back {
    display: inline-flex;
    gap: var(--space-6);
    padding: 0 var(--space-8);
    color: var(--ink-2);
    font-size: var(--fs-13);
    text-decoration: none;
  }
  .back:hover {
    color: var(--ink);
  }
  h2 {
    margin: 0;
    padding: 0 var(--space-8);
    font-size: var(--fs-17);
    font-weight: 600;
  }
  .group {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .cap {
    padding: 0 var(--space-8) var(--space-4);
    color: var(--label);
    font-size: var(--fs-11);
    font-weight: 600;
    letter-spacing: 0.04em;
  }
  .store {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    padding: 0 var(--space-8) var(--space-8);
  }
  .label {
    color: var(--label);
    font-size: var(--fs-11);
  }
  select {
    min-height: var(--control-h);
    padding: 0 var(--space-8);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    background: var(--surface);
    color: var(--ink);
    font: inherit;
    font-size: var(--fs-13);
  }
  .item {
    display: flex;
    align-items: center;
    gap: var(--space-8);
    min-height: var(--row-h);
    padding: 0 var(--space-8);
    border-radius: var(--radius-6);
    color: var(--ink-2);
    text-decoration: none;
  }
  .item:hover {
    background: var(--hover);
  }
  .item.on,
  .item.on:hover {
    background: var(--accent-weak);
    color: var(--accent-ink);
    font-weight: 600;
  }
  .badge {
    margin-left: auto;
    padding: 0 var(--space-6);
    border-radius: var(--radius-pill);
    background: var(--warning-weak);
    color: var(--warning);
    font-size: var(--fs-11);
    font-weight: 600;
  }
  .note {
    margin: var(--space-8) 0 0;
    padding: 0 var(--space-8);
    color: var(--muted);
    font-size: var(--fs-12);
  }
  .foot {
    margin: auto 0 0;
    padding: 0 var(--space-8);
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--fs-11);
  }
</style>
