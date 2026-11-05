<script lang="ts">
  // A text or date field of the dialogs of a sprint or a release.
  type Props = {
    label: string;
    value: string;
    type?: "text" | "date";
    first?: boolean; // takes the focus when the dialog opens
    oninput: (v: string) => void;
  };
  let { label, value, type = "text", first = false, oninput }: Props = $props();
  const uid = $props.id();
</script>

<div class="field">
  <label for={uid}>{label}</label>
  <input
    id={uid}
    {type}
    autocomplete="off"
    data-autofocus={first ? "" : undefined}
    {value}
    oninput={(e) => oninput(e.currentTarget.value)}
  >
</div>

<style>
  .field {
    display: flex;
    flex: 1 1 150px;
    flex-direction: column;
    gap: var(--space-6);
    min-width: 0;
  }
  label {
    color: var(--ink-2);
    font-size: var(--fs-12);
    font-weight: 600;
  }
  input {
    min-height: var(--control-h-lg);
    padding: 0 var(--space-10);
    border: 1px solid var(--line-input);
    border-radius: var(--radius-8);
    font-size: var(--fs-14);
  }
  input[type="date"] {
    font-family: var(--font-mono);
    font-size: var(--fs-13);
  }
  input:focus-visible {
    outline: 2px solid var(--accent);
  }
</style>
