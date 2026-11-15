<script lang="ts">
  // レイアウトの切り替え(04「検索」のツールバーの左端)。選んでいるものを押した状態(aria-pressed)で出す。
  interface Props {
    // choices は、出すレイアウト(並べる順)。
    choices: { id: string; label: string }[];
    value: string;
    onchange: (id: string) => void;
    // label は、読み上げの名前(まとめ方の切り替えにも使う)。
    label?: string;
  }

  let { choices, value, onchange, label = "レイアウト" }: Props = $props();
</script>

<fieldset class="switch" aria-label={label}>
  {#each choices as c (c.id)}
    <button
      type="button"
      class="seg"
      aria-pressed={c.id === value ? "true" : "false"}
      onclick={() => onchange(c.id)}
    >
      {c.label}
    </button>
  {/each}
</fieldset>

<style>
  .switch {
    display: inline-flex;
    margin: 0;
    min-width: 0;
    padding: 2px;
    gap: 2px;
    border: 1px solid var(--line-input);
    border-radius: 8px;
    background: var(--surface);
  }
  .seg {
    height: 24px;
    padding: 0 10px;
    border: 0;
    border-radius: 6px;
    background: transparent;
    color: var(--muted);
    font-size: 12.5px;
    white-space: nowrap;
  }
  .seg:hover {
    color: var(--ink);
    background: var(--hover);
  }
  .seg[aria-pressed="true"] {
    background: var(--hover);
    color: var(--ink);
    font-weight: 500;
  }
</style>
