import { describe, expect, it } from "vitest";
import {
  appMissingBandText,
  copiedText,
  copyFailedText,
  installRequestText,
  methodChoices,
  methodLine,
  reauthedMessage,
  recheckedText,
  signedInMessage,
  stillUnreadableText,
} from "./text";

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

  it("writes the texts of the app missing band", () => {
    expect(appMissingBandText("work", "acme")).toBe(
      "work: acme に GitHub App が入っていないため、PR と Issue を読めません。",
    );
    expect(installRequestText("acme", "taro")).toBe(
      "acme に GitHub App をインストールしてください(@taro が使います)",
    );
    expect(recheckedText("work", ["a", "b"])).toBe("work: a・b を読めるようになりました");
    expect(copiedText).toBe("コピーしました");
    expect(copyFailedText).toBe("コピーできませんでした。文を選択してコピーしてください");
    expect(stillUnreadableText).toBe("まだ読めません");
  });
});
