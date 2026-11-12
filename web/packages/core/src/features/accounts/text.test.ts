import { describe, expect, it } from "vitest";
import { methodChoices, methodLine, reauthedMessage, signedInMessage } from "./text";

describe("account texts", () => {
  it("lists the four methods in the order of 07, the device flow first", () => {
    expect(methodChoices).toEqual([
      { value: "device", label: "ブラウザでサインイン(デバイスフロー)" },
      { value: "gh", label: "gh CLI のトークン" },
      { value: "env", label: "環境変数" },
      { value: "keychain", label: "OS のキーチェーン" },
    ]);
  });

  it("writes the method line of a row", () => {
    expect(methodLine("device")).toBe("github.com · デバイスフローのトークン");
    expect(methodLine("gh")).toBe("github.com · gh CLI のトークン");
    expect(methodLine("env")).toBe("github.com · 環境変数 GH_TOKEN");
    expect(methodLine("keychain")).toBe("github.com · OS のキーチェーン");
  });

  it("writes the toasts with the names", () => {
    expect(signedInMessage("taro")).toBe("@taro としてサインインしました");
    expect(reauthedMessage({ name: "work", login: "taro" })).toBe(
      "work: @taro として再認証しました",
    );
  });
});
