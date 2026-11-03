import { describe, expect, it } from "vitest";
import { makeResponse } from "../src/router";
import { router, routeCount } from "../src/routes";

describe("routes", () => {
  it("lists users", () => {
    const res = makeResponse();
    expect(router.handle("GET", "/users", { params: {}, query: {} }, res)).toBe(true);
    expect(res.payload).toHaveLength(2);
  });

  it("creates a user", () => {
    const res = makeResponse();
    router.handle("POST", "/users", { params: {}, query: {}, body: { name: "cy" } }, res);
    expect(res.statusCode).toBe(201);
  });

  it("counts routes", () => {
    expect(routeCount()).toBe(4);
  });

  it("returns false for unknown routes", () => {
    expect(router.handle("PUT", "/nope", { params: {}, query: {} }, makeResponse())).toBe(false);
  });
});
