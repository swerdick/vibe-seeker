import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useVenues } from "./useVenues";

beforeEach(() => {
  vi.restoreAllMocks();
});

function mockFetch(venueResponse: Response) {
  return vi.spyOn(globalThis, "fetch").mockImplementation(() =>
    Promise.resolve(venueResponse.clone()),
  );
}

describe("useVenues", () => {
  it("does not fetch when disabled", () => {
    const spy = mockFetch(
      new Response(JSON.stringify({ venues: [], count: 0 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    renderHook(() => useVenues(false));
    expect(spy).not.toHaveBeenCalled();
  });

  it("fetches venues when enabled", async () => {
    const venue = {
      ID: "v1",
      Name: "Test Venue",
      Latitude: 40,
      Longitude: -74,
      Address: "123 Main",
      City: "NYC",
      State: "NY",
      ShowsTracked: 1,
      shows: null,
      vibes: null,
    };
    mockFetch(
      new Response(JSON.stringify({ venues: [venue], count: 1 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const { result } = renderHook(() => useVenues(true));
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    expect(result.current.venues).toHaveLength(1);
    expect(result.current.venues[0].Name).toBe("Test Venue");
  });

  it("handles fetch error", async () => {
    mockFetch(new Response("error", { status: 500 }));

    const { result } = renderHook(() => useVenues(true));
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    expect(result.current.venues).toEqual([]);
    expect(result.current.error).toBeTruthy();
  });
});
