import { renderHook, waitFor, act } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAuth } from "./useAuth";

beforeEach(() => {
  vi.restoreAllMocks();
});

function mockFetch(meResponse: Response, logoutResponse?: Response) {
  return vi.spyOn(globalThis, "fetch").mockImplementation((input) => {
    const url =
      typeof input === "string" ? input : input instanceof URL ? input.href : (input as Request).url;
    if (url === "/api/auth/me") return Promise.resolve(meResponse.clone());
    if (url === "/api/auth/logout")
      return Promise.resolve(logoutResponse ?? new Response(null, { status: 204 }));
    return Promise.resolve(new Response("not found", { status: 404 }));
  });
}

describe("useAuth", () => {
  it("returns user after successful auth check", async () => {
    mockFetch(
      new Response(
        JSON.stringify({ spotify_id: "s123", display_name: "Test" }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    const { result } = renderHook(() => useAuth());
    expect(result.current.loading).toBe(true);

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    expect(result.current.authenticated).toBe(true);
    expect(result.current.user?.display_name).toBe("Test");
  });

  it("returns unauthenticated after failed auth check", async () => {
    mockFetch(new Response("unauthorized", { status: 401 }));

    const { result } = renderHook(() => useAuth());
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    expect(result.current.authenticated).toBe(false);
    expect(result.current.user).toBeNull();
  });

  it("logout clears user", async () => {
    mockFetch(
      new Response(
        JSON.stringify({ spotify_id: "s123", display_name: "Test" }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    const { result } = renderHook(() => useAuth());
    await waitFor(() => {
      expect(result.current.authenticated).toBe(true);
    });

    await act(() => result.current.logout());
    expect(result.current.authenticated).toBe(false);
    expect(result.current.user).toBeNull();
  });
});
