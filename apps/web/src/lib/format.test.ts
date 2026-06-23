import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  dateInputToEndOfDayISO,
  daysUntil,
  formatDurationBetween,
  formatDurationMs,
  localDateInputValue,
  relativeTime,
  timeAgo,
} from "./format";

describe("timeAgo", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-14T12:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns just now for future dates", () => {
    expect(timeAgo("2026-06-14T12:01:00Z")).toBe("just now");
  });

  it("returns seconds for recent past", () => {
    expect(timeAgo("2026-06-14T11:59:30Z")).toBe("30s ago");
  });

  it("returns minutes", () => {
    expect(timeAgo("2026-06-14T11:45:00Z")).toBe("15m ago");
  });

  it("returns hours", () => {
    expect(timeAgo("2026-06-14T08:00:00Z")).toBe("4h ago");
  });

  it("returns days", () => {
    expect(timeAgo("2026-06-10T12:00:00Z")).toBe("4d ago");
  });

  it("returns locale date string for dates older than 30 days", () => {
    const result = timeAgo("2026-01-01T12:00:00Z");
    expect(result).toBe(new Date("2026-01-01T12:00:00Z").toLocaleDateString());
  });
});

describe("relativeTime", () => {
  it("is an alias for timeAgo", () => {
    expect(relativeTime).toBe(timeAgo);
  });
});

describe("formatDurationMs", () => {
  it("formats sub-minute durations", () => {
    expect(formatDurationMs(45_000)).toBe("45s");
  });

  it("formats minute durations", () => {
    expect(formatDurationMs(150_000)).toBe("2m 30s");
  });
});

describe("formatDurationBetween", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-14T12:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns dash when start is missing", () => {
    expect(formatDurationBetween()).toBe("-");
  });

  it("formats duration between two timestamps", () => {
    expect(formatDurationBetween("2026-06-14T11:58:00Z", "2026-06-14T12:00:00Z")).toBe("2m 0s");
  });

  it("uses now when end is omitted", () => {
    expect(formatDurationBetween("2026-06-14T11:59:00Z")).toBe("1m 0s");
  });

  it("returns 0s for negative duration", () => {
    expect(formatDurationBetween("2026-06-14T12:01:00Z", "2026-06-14T12:00:00Z")).toBe("0s");
  });
});

describe("daysUntil", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-14T12:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns positive days for future dates", () => {
    expect(daysUntil("2026-06-20T12:00:00Z")).toBe(6);
  });

  it("returns negative days for past dates", () => {
    expect(daysUntil("2026-06-10T12:00:00Z")).toBe(-4);
  });
});

describe("localDateInputValue", () => {
  it("formats the local calendar date", () => {
    const date = new Date(2026, 5, 19, 15, 30);
    expect(localDateInputValue(date)).toBe("2026-06-19");
  });
});

describe("dateInputToEndOfDayISO", () => {
  it("serializes to local end of day instead of UTC midnight", () => {
    const iso = dateInputToEndOfDayISO("2026-06-19");
    const parsed = new Date(iso);
    expect(parsed.getFullYear()).toBe(2026);
    expect(parsed.getMonth()).toBe(5);
    expect(parsed.getDate()).toBe(19);
    expect(parsed.getHours()).toBe(23);
    expect(parsed.getMinutes()).toBe(59);
    expect(parsed.getSeconds()).toBe(59);
  });
});
