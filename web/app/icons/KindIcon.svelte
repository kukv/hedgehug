<script lang="ts" module>
  // Paths are the ones of the Foundations artboard (16px grid, 1.4 stroke).
  const kinds: Record<string, string> = {
    project: "M2.5 4.5h4l1.5 1.5h5.5v6.5h-11z",
    epic: "M8 2.5l5.5 2.8L8 8.1 2.5 5.3zM2.5 8.2L8 11l5.5-2.8M2.5 11L8 13.8l5.5-2.8",
    story: "M4.5 2.5h7v11L8 11.2l-3.5 2.3z",
    task: "M3 3h10v10H3zM5.6 8.1l1.7 1.7 3.1-3.4",
    step: "M8 3a5 5 0 1 0 0 10 5 5 0 0 0 0-10zM6.2 8.1l1.4 1.4 2.4-2.8",
    bug: "M5.5 7a2.5 2.5 0 0 1 5 0v3a2.5 2.5 0 0 1-5 0zM8 7v5.5M5.5 9H2.8M13.2 9h-2.7M6 5.2L4.4 3.6M10 5.2l1.6-1.6M5.6 11.6l-1.8 1.4M10.4 11.6l1.8 1.4",
  };
  // A name that is not a kind (a task of an old file) gets a plain square.
  const workShape = "M3 3h10v10H3z";
</script>

<script lang="ts">
  type Props = { kind: string; size?: number };
  let { kind, size = 16 }: Props = $props();

  // The shape is fixed per kind; bug is red and the others are the icon color.
  const d = $derived(kinds[kind] ?? workShape);
  const tint = $derived(kind === "bug" ? "var(--danger)" : "var(--icon)");
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 16 16"
  data-kind={kind}
  style:--k={tint}
  aria-hidden="true"
>
  <path
    {d}
    fill="none"
    stroke="currentColor"
    stroke-width="1.4"
    stroke-linejoin="round"
    stroke-linecap="round"
  />
</svg>

<style>
  svg {
    flex: 0 0 auto;
    color: var(--k);
  }
</style>
