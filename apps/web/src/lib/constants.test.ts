import { describe, expect, it } from "vitest";
import { statusDotColor, statusDotPulse, statusVariant } from "./constants";

describe("statusVariant", () => {
  it.each([
    ["running", "success"],
    ["success", "success"],
    ["Running", "success"],
    ["Ready", "success"],
    ["Bound", "success"],
    ["Succeeded", "secondary"],
    ["Completed", "secondary"],
    ["stopped", "outline"],
    ["idle", "outline"],
    ["not deployed", "outline"],
    ["building", "warning"],
    ["deploying", "warning"],
    ["restarting", "warning"],
    ["stopping", "warning"],
    ["queued", "warning"],
    ["Pending", "warning"],
    ["pending", "warning"],
    ["partial", "warning"],
    ["error", "destructive"],
    ["failed", "destructive"],
    ["Failed", "destructive"],
    ["Evicted", "destructive"],
    ["unknown", "secondary"],
  ] as const)("maps %s to %s", (status, expected) => {
    expect(statusVariant(status)).toBe(expected);
  });
});

describe("statusDotColor", () => {
  it.each([
    ["running", "bg-success"],
    ["Running", "bg-success"],
    ["Ready", "bg-success"],
    ["Bound", "bg-success"],
    ["building", "bg-warning"],
    ["deploying", "bg-warning"],
    ["restarting", "bg-warning"],
    ["stopping", "bg-warning"],
    ["Pending", "bg-warning"],
    ["pending", "bg-warning"],
    ["queued", "bg-warning"],
    ["error", "bg-destructive"],
    ["failed", "bg-destructive"],
    ["Failed", "bg-destructive"],
    ["Evicted", "bg-destructive"],
    ["stopped", "bg-muted-foreground/40"],
    ["idle", "bg-muted-foreground/40"],
    ["not deployed", "bg-muted-foreground/40"],
    ["Succeeded", "bg-muted-foreground/40"],
    ["Completed", "bg-muted-foreground/40"],
    ["unknown", "bg-muted-foreground/40"],
  ] as const)("maps %s to %s", (status, expected) => {
    expect(statusDotColor(status)).toBe(expected);
  });
});

describe("statusDotPulse", () => {
  it.each([
    "building",
    "deploying",
    "restarting",
    "stopping",
    "Pending",
    "pending",
    "queued",
  ])("returns true for %s", (status) => {
    expect(statusDotPulse(status)).toBe(true);
  });

  it.each(["running", "stopped", "error", "unknown"])("returns false for %s", (status) => {
    expect(statusDotPulse(status)).toBe(false);
  });
});
