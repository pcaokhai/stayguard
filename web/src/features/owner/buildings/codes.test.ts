import { describe, expect, it } from "vitest";
import { between, buildingCodes, codeRange } from "./codes";

describe("room code previews", () => {
  it("numbers rooms by floor", () => {
    expect(buildingCodes("E", 2, 2)).toEqual(["E101", "E102", "E201", "E202"]);
  });
  it("counts up from a start code", () => {
    expect(codeRange("A401", 3)).toEqual(["A401", "A402", "A403"]);
  });
  it("lists a range and rejects a bad one", () => {
    expect(between("A107", "A109")).toEqual(["A107", "A108", "A109"]);
    expect(between("A109", "A107")).toEqual([]);
    expect(between("A107", "B109")).toEqual([]);
  });
});
