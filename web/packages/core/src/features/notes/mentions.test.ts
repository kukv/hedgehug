import { describe, expect, it } from "vitest";
import { appendMention, mentionTargets } from "./mentions";

const members = [
  { login: "taro", name: "太郎" },
  { login: "Hanako", name: "花子" },
  { login: "jiro", name: "次郎" },
];

describe("mentions", () => {
  it("lists the logins of the members other than me, in the member order", () => {
    expect(mentionTargets(members, "hanako")).toEqual(["taro", "jiro"]);
    expect(mentionTargets(members, "")).toEqual(["taro", "Hanako", "jiro"]);
  });

  it("appends the login and a space to the end of the text", () => {
    expect(appendMention("", "taro")).toBe("@taro ");
    expect(appendMention("見てください ", "taro")).toBe("見てください @taro ");
  });
});
