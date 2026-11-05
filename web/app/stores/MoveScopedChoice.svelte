<script lang="ts">
  // The choice of what the projects moved have of their own, and what carrying it leaves out.
  let {
    dropped,
    scoped,
    onscoped,
  }: {
    dropped: string[];
    scoped: "carry" | "drop" | undefined;
    onscoped: (v: "carry" | "drop") => void;
  } = $props();
  const uid = $props.id();
</script>

<div class="part">
  <div class="field" role="radiogroup" aria-labelledby="{uid}-scoped-lbl">
    <span class="lbl" id="{uid}-scoped-lbl">プロジェクトの設定（表示と自動化）</span>
    <label class="plain">
      <input
        type="radio"
        name="{uid}-scoped"
        checked={scoped === "carry"}
        onchange={() => onscoped("carry")}
      >
      持っていく
    </label>
    <label class="plain">
      <input
        type="radio"
        name="{uid}-scoped"
        checked={scoped === "drop"}
        onchange={() => onscoped("drop")}
      >
      持っていかない
    </label>
  </div>
  {#if dropped.length > 0}
    <div class="dropped">
      <span class="lbl">持っていくと外れるもの</span>
      <ul>
        {#each dropped as d (d)}
          <li>{d}</li>
        {/each}
      </ul>
    </div>
  {/if}
</div>

<style>
  .part {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding-top: var(--space-12);
    border-top: 1px solid var(--line);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }
  .lbl {
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  .plain {
    display: flex;
    gap: var(--space-8);
    align-items: center;
    font-size: var(--fs-13);
  }
  .dropped ul {
    margin: var(--space-4) 0 0;
    padding-left: var(--space-20);
    font-size: var(--fs-13);
  }
</style>
