import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useVibes } from "./useVibes";

beforeEach(() => {
  vi.restoreAllMocks();
});

function mockFetch(vibeResponse: Response) {
  return vi.spyOn(globalThis, "fetch").mockImplementation(() =>
    Promise.resolve(vibeResponse.clone()),
  );
}

describe("useVibes", () => {
  it("does not fetch when disabled", () => {
    const spy = mockFetch(
      new Response(JSON.stringify({ genres: { rock: 1 }, genre_count: 1 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    renderHook(() => useVibes(false));
    expect(spy).not.toHaveBeenCalled();
  });

  it("fetches genres when enabled", async () => {
    mockFetch(
      new Response(
        JSON.stringify({ genres: { rock: 1.0, indie: 0.5 }, genre_count: 2 }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    const { result } = renderHook(() => useVibes(true));
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    expect(result.current.genres).toEqual({ rock: 1.0, indie: 0.5 });
    expect(result.current.error).toBeNull();
  });

  it("handles fetch error", async () => {
    mockFetch(new Response("error", { status: 500 }));

    const { result } = renderHook(() => useVibes(true));
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    expect(result.current.genres).toBeNull();
    expect(result.current.error).toBeTruthy();
  });
});
