import { beforeEach, describe, expect, it } from "vitest";
import { tokenStore } from "./token-store";

describe("tokenStore", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("returns null for unset tokens", () => {
    expect(tokenStore.getAccess()).toBeNull();
    expect(tokenStore.getRefresh()).toBeNull();
  });

  it("persists both tokens under the canonical keys", () => {
    tokenStore.set("access-1", "refresh-1");

    expect(tokenStore.getAccess()).toBe("access-1");
    expect(tokenStore.getRefresh()).toBe("refresh-1");
    expect(localStorage.getItem("orkai_token")).toBe("access-1");
    expect(localStorage.getItem("orkai_refresh")).toBe("refresh-1");
  });

  it("clears both tokens", () => {
    tokenStore.set("access-1", "refresh-1");

    tokenStore.clear();

    expect(tokenStore.getAccess()).toBeNull();
    expect(tokenStore.getRefresh()).toBeNull();
  });
});
