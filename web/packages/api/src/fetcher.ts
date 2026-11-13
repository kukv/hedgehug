// orval が作る呼び出しが通る fetch。トークンを付け、失敗した応答は ApiError にする。

let token = "";

export function setToken(value: string): void {
  token = value;
}

// authHeader は、API に付ける Authorization の値(出来事の読み続けも使う)。
export function authHeader(): string {
  return `Bearer ${token}`;
}

// ApiError の message は、応答の detail(断りの文言。huma のエラーの本文)があればそれ、なければ状態の行。
export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

function detailOf(text: string): string | null {
  try {
    const body: unknown = JSON.parse(text);
    if (body && typeof body === "object" && "detail" in body && typeof body.detail === "string") {
      return body.detail;
    }
  } catch {
    // JSON でない本文(401 の "unauthorized" など)は、状態の行にする。
  }
  return null;
}

// Restored は、削除を取り消した保存の応答が知らせる 1 件。id は項目の ID、name は設定の値の名前、comment はコメントの持ち主の表示 ID。
export type Restored = { kind: "id" | "name" | "comment"; text: string };

let restoredListener: ((restored: Restored[]) => void) | null = null;

// onRestored は、削除を取り消した保存の応答(Hedgehug-Restored 系のヘッダー)を受ける受け手を 1 つ置く。
export function onRestored(listener: ((restored: Restored[]) => void) | null): void {
  restoredListener = listener;
}

// 不正な % の並びは、保存の成功を失敗にしないよう、そのままの値を使う。
function decodeName(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

const restoredHeaders: { name: string; kind: Restored["kind"]; decode: (v: string) => string }[] = [
  { name: "Hedgehug-Restored", kind: "id", decode: (v) => v },
  { name: "Hedgehug-Restored-Name", kind: "name", decode: decodeName },
  { name: "Hedgehug-Restored-Comment", kind: "comment", decode: (v) => v },
];

function notifyRestored(res: Response): void {
  const restored = restoredHeaders.flatMap(({ name, kind, decode }) =>
    (res.headers.get(name) ?? "")
      .split(",")
      .map((v) => v.trim())
      .filter((v) => v !== "")
      .map((v) => ({ kind, text: decode(v) })),
  );
  if (restored.length > 0) restoredListener?.(restored);
}

export async function fetcher<T>(url: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  headers.set("Authorization", authHeader());
  const res = await fetch(url, { ...init, headers });
  const text = await res.text();
  if (!res.ok) {
    throw new ApiError(res.status, detailOf(text) ?? `${res.status} ${res.statusText}`);
  }
  notifyRestored(res);
  return (text === "" ? undefined : JSON.parse(text)) as T;
}
