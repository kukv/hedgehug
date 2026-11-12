import { NotFound, Refused } from "@hedgehug/core";
import { ApiError } from "./fetcher";

// call は、API の呼び出しの失敗を core の言葉にする(A2 の計画の決定 13)。
// 422 は Refused(message は detail の仕様の文言)、404 は NotFound、ほかはそのまま投げる。
export async function call<T>(fn: () => Promise<T>): Promise<T> {
  try {
    return await fn();
  } catch (e) {
    if (e instanceof ApiError && e.status === 422) throw new Refused(e.message);
    if (e instanceof ApiError && e.status === 404) throw new NotFound(e.message);
    throw e;
  }
}
