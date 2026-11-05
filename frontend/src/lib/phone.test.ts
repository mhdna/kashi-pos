import { describe, expect, it } from "vitest";
import { formatLebanesePhone, localLebanesePhone, normalizeLebanesePhone } from "./phone";

describe("formatLebanesePhone", () => {
  it("does not prepend zero to an eight-digit Lebanese mobile number", () => {
    expect(formatLebanesePhone("78834243")).toBe("(78) 834-243");
    expect(formatLebanesePhone("+96178834243")).toBe("(78) 834-243");
  });

  it("restores the trunk zero for seven-digit Lebanese national numbers", () => {
    expect(formatLebanesePhone("+9613123456")).toBe("(03) 123-456");
  });
});

describe("localLebanesePhone", () => {
  it("gives 03 numbers and landlines their 0 back, so they show right", () => {
    expect(localLebanesePhone("+9613927807")).toBe("03927807");
    expect(formatLebanesePhone(localLebanesePhone("+9613927807"))).toBe("(03) 927-807");
    expect(localLebanesePhone("+9611234567")).toBe("01234567");
  });

  it("leaves 8-digit mobiles and other countries alone", () => {
    expect(localLebanesePhone("+96178834243")).toBe("78834243");
    expect(localLebanesePhone("+33612345678")).toBe("+33612345678");
    expect(localLebanesePhone("00000000")).toBe("00000000");
  });

  it("saves as the same number", () => {
    for (const phone of ["+9613927807", "+9611234567", "+96178834243"]) {
      expect(normalizeLebanesePhone(localLebanesePhone(phone))).toBe(phone);
    }
  });
});
