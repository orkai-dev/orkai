import { describe, expect, it } from "vitest";
import { queryKeysForTable } from "./use-event-source";

describe("queryKeysForTable", () => {
  it.each([
    ["applications", [["apps"], ["projects"]]],
    ["deployments", [["apps"], ["deployments"]]],
    ["domains", [["apps"]]],
    ["managed_databases", [["databases"], ["projects"]]],
    ["projects", [["projects"]]],
    ["server_nodes", [["nodes"], ["cluster"]]],
    ["shared_resources", [["resources"]]],
    ["alerts", [["monitoring"]]],
    ["cron_jobs", [["cronjobs"], ["projects"]]],
    ["cron_job_runs", [["cronjobs"]]],
  ] as const)("maps %s to expected query keys", (table, expected) => {
    expect(queryKeysForTable(table)).toEqual(expected);
  });

  it("returns empty array for unknown tables", () => {
    expect(queryKeysForTable("unknown_table")).toEqual([]);
  });
});
