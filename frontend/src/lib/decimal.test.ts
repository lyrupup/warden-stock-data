import { describe, expect, it } from "vitest";
import { changeColor, formatPct, toNumber } from "./decimal";

describe("decimal", () => {
  it("toNumber converts string decimals", () => {
    expect(toNumber("10.5")).toBe(10.5);
    expect(toNumber(null)).toBe(0);
  });

  it("formatPct formats percentage", () => {
    expect(formatPct("1.234")).toBe("1.23%");
  });

  it("changeColor returns correct class", () => {
    expect(changeColor("1")).toContain("red");
    expect(changeColor("-1")).toContain("green");
    expect(changeColor("0")).toContain("muted");
  });
});
