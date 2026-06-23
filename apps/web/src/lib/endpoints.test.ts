import { describe, expect, it } from "vitest";
import { API_BASE, API_VERSION, endpoints } from "./endpoints";

describe("endpoints", () => {
  it("builds paths from the configured API version", () => {
    expect(API_VERSION).toBe("v1");
    expect(API_BASE).toBe("/api/v1");
  });

  it("builds auth endpoints", () => {
    expect(endpoints.auth.login()).toBe("/api/v1/auth/login");
    expect(endpoints.auth.refresh()).toBe("/api/v1/auth/refresh");
    expect(endpoints.auth.me()).toBe("/api/v1/auth/me");
  });

  it("builds parameterized app endpoints", () => {
    expect(endpoints.apps.detail("abc")).toBe("/api/v1/apps/abc");
    expect(endpoints.apps.deploy("abc")).toBe("/api/v1/apps/abc/deploy");
    expect(endpoints.apps.podEvents("abc", "pod-1")).toBe("/api/v1/apps/abc/pods/pod-1/events");
  });

  it("builds nested resource endpoints", () => {
    expect(endpoints.projects.apps("p1")).toBe("/api/v1/projects/p1/apps");
    expect(endpoints.deployments.cancel("d1")).toBe("/api/v1/deployments/d1/cancel");
    expect(endpoints.domains.detail("dom1")).toBe("/api/v1/domains/dom1");
  });
});
