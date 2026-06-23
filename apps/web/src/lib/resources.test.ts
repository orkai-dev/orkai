import { describe, expect, it } from "vitest";
import {
  fmtCPU,
  fmtMem,
  parseResourceValue,
  parseToMiB,
  parseToMillicores,
  pctNumber,
  pctString,
} from "./resources";

describe("parseToMillicores", () => {
  it.each([
    ["", 0],
    ["250m", 250],
    ["500000n", 0.5],
    ["2", 2000],
    [" 100m ", 100],
  ] as const)('parses "%s" to %s', (raw, expected) => {
    expect(parseToMillicores(raw)).toBe(expected);
  });
});

describe("parseToMiB", () => {
  it.each([
    ["", 0],
    ["512Ki", 0.5],
    ["256Mi", 256],
    ["2Gi", 2048],
  ] as const)('parses "%s" to %s', (raw, expected) => {
    expect(parseToMiB(raw)).toBe(expected);
  });
});

describe("fmtCPU", () => {
  it("formats sub-core millicores", () => {
    expect(fmtCPU(250)).toBe("250m");
  });

  it("formats cores", () => {
    expect(fmtCPU(1500)).toBe("1.5 cores");
  });
});

describe("fmtMem", () => {
  it("formats KiB", () => {
    expect(fmtMem(0.5)).toBe("512 Ki");
  });

  it("formats MiB", () => {
    expect(fmtMem(256)).toBe("256 Mi");
  });

  it("formats GiB", () => {
    expect(fmtMem(2048)).toBe("2.0 Gi");
  });
});

describe("parseResourceValue", () => {
  it.each([
    ["", 0],
    ["280m", 280],
    ["2", 2000],
    ["1500m", 1500],
    ["1536Mi", 1536],
    ["2Gi", 2048],
    ["123456Ki", 120.5625],
  ] as const)('parses "%s"', (val, expected) => {
    expect(parseResourceValue(val)).toBeCloseTo(expected, 2);
  });
});

describe("pctString", () => {
  it("returns percentage string", () => {
    expect(pctString("500m", "1000m")).toBe("50%");
  });

  it("returns 0% when total is zero", () => {
    expect(pctString("100m", "")).toBe("0%");
  });
});

describe("pctNumber", () => {
  it("returns capped percentage", () => {
    expect(pctNumber("800m", "1000m")).toBe(80);
  });

  it("returns 0 when total is zero", () => {
    expect(pctNumber("100m", "0")).toBe(0);
  });

  it("caps at 100", () => {
    expect(pctNumber("2000m", "1000m")).toBe(100);
  });
});
