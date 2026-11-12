// 小さな購読の仕組み。Svelte のストアの約束(subscribe が今の値をすぐ渡し、止める関数を返す)に従うので、
// Svelte の画面では $store と書ける。core は Svelte を import しない。

export type Subscriber<T> = (value: T) => void;
export type Unsubscriber = () => void;

export interface Readable<T> {
  subscribe(run: Subscriber<T>): Unsubscriber;
}

export interface Writable<T> extends Readable<T> {
  set(value: T): void;
  update(fn: (value: T) => T): void;
}

export function writable<T>(initial: T): Writable<T> {
  let value = initial;
  const subscribers = new Set<Subscriber<T>>();
  const set = (next: T) => {
    if (Object.is(next, value)) return;
    value = next;
    for (const run of [...subscribers]) run(value);
  };
  return {
    subscribe(run) {
      subscribers.add(run);
      run(value);
      return () => {
        subscribers.delete(run);
      };
    },
    set,
    update: (fn) => set(fn(value)),
  };
}

export function readonly<T>(store: Writable<T>): Readable<T> {
  return { subscribe: store.subscribe };
}

export function get<T>(store: Readable<T>): T {
  let value: T | undefined;
  store.subscribe((v) => {
    value = v;
  })();
  return value as T;
}
