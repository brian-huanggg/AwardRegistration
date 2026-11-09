import cases from "testdata/password.json";
import { loadCommonPasswords, parseCommonList } from "./common-passwords";
import { checkPassword, empNoForPolicy, foldCase, PASSWORD_RULES } from "./password-policy";

describe("checkPassword: testdata/password.json (same cases as the backend)", () => {
  it.each(cases.cases.map((c) => [c.id, c] as const))("%s", async (_id, c) => {
    const isCommon = await loadCommonPasswords();
    expect(checkPassword(c.password, c.empNo, isCommon)).toEqual(c.wantRules);
  });

  it("covers every rule id at least once", () => {
    const seen = new Set(cases.cases.flatMap((c) => c.wantRules));
    expect(PASSWORD_RULES.filter((r) => !seen.has(r))).toEqual([]);
  });
});

describe("checkPassword: client specifics", () => {
  it("without the blocklist never reports COMMON_PASSWORD, the rest unchanged", () => {
    for (const c of cases.cases) {
      expect(checkPassword(c.password, c.empNo)).toEqual(
        c.wantRules.filter((r) => r !== "COMMON_PASSWORD"),
      );
    }
  });

  it("counts a lone surrogate as one code point outside printable ASCII", () => {
    expect(checkPassword("Correct-Hors\uD800", "")).toEqual(["CHARSET"]);
  });

  it("folds case like Go strings.ToLower, not toLowerCase (U+0130, U+212A)", () => {
    expect(foldCase("PASSİK")).toBe("passik");
    // U+212A (Kelvin) folds to k, so "jkl;" is a keyboard run on the server too
    expect(checkPassword("Zq-JKL;-Mx81", "")).toEqual(["CHARSET", "KEYBOARD_RUN"]);
  });
});

describe("empNoForPolicy", () => {
  it.each([
    ["089290", "089290"],
    ["08929", ""],
    ["0892901", ""],
    ["08929a", ""],
    ["０８９２９０", ""],
  ])("%j → %j", (input, want) => expect(empNoForPolicy(input)).toBe(want));
});

describe("common-password list", () => {
  it("loads once as a lazy chunk and matches whole passwords ignoring case", async () => {
    const a = loadCommonPasswords();
    expect(loadCommonPasswords()).toBe(a);
    const isCommon = await a;
    expect(isCommon("password")).toBe(true);
    expect(isCommon("PassWord")).toBe(true);
    expect(isCommon(" password")).toBe(false);
    expect(isCommon("password1x")).toBe(false);
  });

  it("parses CRLF lines and skips empty ones", () => {
    const isCommon = parseCommonList("Alpha\r\n\r\nbeta\n");
    expect(isCommon("alpha")).toBe(true);
    expect(isCommon("BETA")).toBe(true);
    expect(isCommon("")).toBe(false);
  });
});
