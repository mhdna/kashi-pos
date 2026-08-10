import { describe, expect, it } from "vitest";
import { formatLebanesePhone } from "./phone";

describe("formatLebanesePhone", () => {
  it("does not prepend zero to an eight-digit Lebanese mobile number", () => {
    expect(formatLebanesePhone("78834243")).toBe("(78) 834-243");
    expect(formatLebanesePhone("+96178834243")).toBe("(78) 834-243");
  });

  it("restores the trunk zero for seven-digit Lebanese national numbers", () => {
    expect(formatLebanesePhone("+9613123456")).toBe("(03) 123-456");
  });
});
