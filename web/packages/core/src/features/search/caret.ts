// カーソルの位置の語のキーと打った値を読む(E1-1)。クエリを解析して一致を決めるのはサーバーの
// domain/query だけで、ここは候補を出すために語の始まりを見るだけ。語の切り方とキーの形は
// internal/domain/query/parse.go と同じ(同じ入力の表で caret.test.ts がテストする)。

export interface CaretTerm {
  key: string;
  not: boolean;
  // start は打った値の始まり(最後の `,` と空白のあと。開き引用符を含む)、end はカーソル。
  start: number;
  end: number;
  // typed は、start からカーソルまでから引用符を除いたもの。
  typed: string;
}

const keyPattern = /^([A-Za-z][A-Za-z0-9_]*)(:|<=|>=|<|>)/;
// Go の unicode.IsSpace との違いは U+0085 と U+FEFF だけ(JS の \s は U+FEFF を空白にし、U+0085 は空白にしない)。
const space = /\s/u;

interface Word {
  start: number;
  end: number;
}

// words は、空白で語に切る。引用符のあいだの空白は区切らず、閉じていない引用符は末尾までを 1 つの語にする。
function words(text: string): Word[] {
  const out: Word[] = [];
  let from = -1;
  let inQuote = false;
  let i = 0;
  for (const ch of text) {
    if (ch === '"') {
      inQuote = !inQuote;
      if (from < 0) from = i;
    } else if (space.test(ch) && !inQuote) {
      if (from >= 0) out.push({ start: from, end: i });
      from = -1;
    } else if (from < 0) {
      from = i;
    }
    i += ch.length;
  }
  if (from >= 0) out.push({ start: from, end: i });
  return out;
}

// keyOf は、語 raw の頭の `-`(否定。`-` だけの語は否定にしない)と、「キー + 演算子」の一致。キーの形でなければ null。
function keyOf(raw: string): { not: boolean; m: RegExpExecArray } | null {
  const not = raw.length > 1 && raw.startsWith("-");
  const m = keyPattern.exec(not ? raw.slice(1) : raw);
  return m ? { not, m } : null;
}

// hasKeyTerm は、キーの形の語が 1 つ以上あるか(サーバーの解析で「キーのある語」になるもの)。
export function hasKeyTerm(text: string): boolean {
  return words(text).some((w) => keyOf(text.slice(w.start, w.end)) !== null);
}

// caretTerm は、カーソルの位置の語が「キー + 演算子」で始まり、カーソルがその演算子のあとにあるとき、その
// キーと打った値を返す。それ以外(自由な語・空白の上・キーの綴りの途中)は null。
export function caretTerm(text: string, caret: number): CaretTerm | null {
  const word = words(text).find((w) => w.start <= caret && caret <= w.end);
  if (!word) return null;
  const key = keyOf(text.slice(word.start, word.end));
  if (!key) return null;
  const { not, m } = key;
  const valuesFrom = word.start + (not ? 1 : 0) + m[0].length;
  if (caret < valuesFrom) return null;
  const before = text.slice(valuesFrom, caret);
  const afterComma = before.slice(before.lastIndexOf(",") + 1);
  const lead = afterComma.length - afterComma.trimStart().length;
  return {
    key: (m[1] ?? "").toLowerCase(),
    not,
    start: caret - afterComma.length + lead,
    end: caret,
    typed: afterComma.replaceAll('"', "").trim(),
  };
}
