<script lang="ts">
  // 一覧の行の先頭に置く、選択のチェック(F2b1-4)。見える文字はなく、label は読み上げの名前(「{ID} を選択」)。
  // 閲覧のみの Store でも選べる。選べない行(削除済みなど)だけ disabled にする。
  interface Props {
    label: string;
    checked: boolean;
    onchange: (checked: boolean) => void;
    disabled?: boolean;
  }

  let { label, checked, onchange, disabled = false }: Props = $props();
</script>

<input
  type="checkbox"
  aria-label={label}
  {checked}
  {disabled}
  onchange={(e) => !disabled && onchange(e.currentTarget.checked)}
>
