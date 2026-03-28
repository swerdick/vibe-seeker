import { renderHook, waitFor, act } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useSyncAction } from "./useSyncAction";

beforeEach(() => {
  vi.restoreAllMocks();
});

function mockFetch(response: Response) {
  return vi.spyOn(globalThis, "fetch").mockImplementation(() =>
    Promise.resolve(response.clone()),
  );
}

describe("useSyncAction", () => {
  it("executes POST and returns result", async () => {
    mockFetch(
      new Response(JSON.stringify({ synced: true, genre_count: 5 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    const refetch = vi.fn();

    const { result } = renderHook(() =>
      useSyncAction<{ genre_count: number }>("/api/vibe/sync", {
        errorMessage: "Sync failed.",
        refetch,
      }),
    );

    act(() => result.current.execute());
    expect(result.current.syncing).toBe(true);

    await waitFor(() => {
      expect(result.current.syncing).toBe(false);
    });
    expect(result.current.result?.genre_count).toBe(5);
    expect(result.current.error).toBeNull();
    expect(refetch).toHaveBeenCalled();
  });

  it("sets error on failure", async () => {
    mockFetch(new Response("error", { status: 502 }));

    const { result } = renderHook(() =>
      useSyncAction("/api/vibe/sync", {
        errorMessage: "Sync failed.",
      }),
    );

    act(() => result.current.execute());
    await waitFor(() => {
      expect(result.current.syncing).toBe(false);
    });
    expect(result.current.error).toBe("Sync failed.");
    expect(result.current.result).toBeNull();
  });
});
