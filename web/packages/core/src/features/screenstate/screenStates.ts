import type { ScreenStatesPort } from "../../port/search";

// 画面の状態(E1-16)。ブラウザのタブで最初の read だけサーバーから読み、以後はタブの中の値を使う。
// write はタブの中の値を替え、サーバーへ書く(書けなくてもタブの中の値は変わる)。

export interface ScreenStates {
  // read は、読めない・まだない・壊れた JSON のときは fallback。
  read<T>(screen: string, fallback: T): Promise<T>;
  write<T>(screen: string, value: T): void;
}

export function createScreenStates(port: ScreenStatesPort): ScreenStates {
  // known は、画面ごとの、タブの中の値(JSON の文字列。まだ読めていなければ読み込み中の約束)。
  const known = new Map<string, Promise<string | null> | string>();

  const load = (screen: string): Promise<string | null> => {
    const have = known.get(screen);
    if (typeof have === "string") return Promise.resolve(have);
    if (have) return have;
    const p = port.get(screen).catch(() => null);
    known.set(screen, p);
    return p;
  };

  return {
    async read<T>(screen: string, fallback: T): Promise<T> {
      const body = await load(screen);
      // 読み込み中に write があれば、書いた値が先(known は文字列に替わっている)。
      const now = known.get(screen);
      const text = typeof now === "string" ? now : body;
      if (text === null) return fallback;
      try {
        return JSON.parse(text) as T;
      } catch {
        return fallback;
      }
    },
    write<T>(screen: string, value: T) {
      const body = JSON.stringify(value);
      known.set(screen, body);
      // 書けなくてもよい: 画面の状態は復元のための便宜で、タブの中の値を使い続けるため画面は困らない
      // (次に書けたときに追いつく)。失敗を出すと、入力のたびにトーストが出てしまう。
      port.put(screen, body).catch(() => {});
    },
  };
}
