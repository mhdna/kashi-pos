import { describe, expect, it } from "vitest";
import { dayKind, parseDay, perVisitor, weekdayShort } from "./monthlyDays";

describe("parseDay", () => {
  it("reads dd/mm/yyyy", () => {
    expect(parseDay("13/11/2026")?.getTime()).toBe(new Date(2026, 10, 13).getTime());
    expect(parseDay("1/2/2026")?.getTime()).toBe(new Date(2026, 1, 1).getTime());
  });

  it("refuses anything that is not a real day", () => {
    for (const bad of ["", "2026-11-13", "31/02/2026", "00/11/2026", "13/13/2026", "13/11/26", "a/b/c"]) {
      expect(parseDay(bad)).toBeNull();
    }
  });

  it("accepts a leap day only in a leap year", () => {
    expect(parseDay("29/02/2028")).not.toBeNull();
    expect(parseDay("29/02/2026")).toBeNull();
  });
});

describe("weekdayShort", () => {
  it("names the weekday", () => {
    expect(weekdayShort("13/11/2026")).toBe("Fri");
    expect(weekdayShort("14/11/2026")).toBe("Sat");
    expect(weekdayShort("15/11/2026")).toBe("Sun");
    expect(weekdayShort("16/11/2026")).toBe("Mon");
  });

  it("is empty for a non-day", () => {
    expect(weekdayShort("nope")).toBe("");
  });

  it("agrees with the calendar for every day of a year", () => {
    const names = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
    for (let d = new Date(2026, 0, 1); d.getFullYear() === 2026; d.setDate(d.getDate() + 1)) {
      const label = `${String(d.getDate()).padStart(2, "0")}/${String(d.getMonth() + 1).padStart(2, "0")}/2026`;
      expect(weekdayShort(label)).toBe(names[d.getDay()]);
    }
  });
});

describe("dayKind", () => {
  it("is past, today or future against today", () => {
    expect(dayKind("12/11/2026", "13/11/2026")).toBe("past");
    expect(dayKind("13/11/2026", "13/11/2026")).toBe("today");
    expect(dayKind("14/11/2026", "13/11/2026")).toBe("future");
  });

  it("compares across month and year ends", () => {
    expect(dayKind("31/12/2026", "01/01/2027")).toBe("past");
    expect(dayKind("01/01/2027", "31/12/2026")).toBe("future");
    expect(dayKind("30/11/2026", "01/12/2026")).toBe("past");
  });

  it("treats a day it cannot read as past, so it is shown in full", () => {
    expect(dayKind("garbage", "13/11/2026")).toBe("past");
    expect(dayKind("13/11/2026", "garbage")).toBe("past");
  });
});

describe("perVisitor", () => {
  it("is the takings divided by the people, rounded", () => {
    expect(perVisitor(288300, 11)).toBe(26209); // 262.09 in cents
    expect(perVisitor(1000, 3)).toBe(333);
    expect(perVisitor(1000, 1)).toBe(1000);
  });

  it("is nothing without visitors", () => {
    for (const v of [0, -1, null, undefined]) expect(perVisitor(5000, v)).toBeNull();
  });

  it("is zero for a day with visitors and no takings", () => {
    expect(perVisitor(0, 40)).toBe(0);
  });

  it("is nothing for a total that is not a number", () => {
    expect(perVisitor(NaN, 5)).toBeNull();
  });
});
