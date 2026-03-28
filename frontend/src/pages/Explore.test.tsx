import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../contexts/AuthContext";
import Explore from "./Explore";

beforeEach(() => {
  vi.restoreAllMocks();
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderExplore() {
  return render(
    <MemoryRouter initialEntries={["/explore"]}>
      <AuthProvider>
        <Routes>
          <Route path="/explore" element={<Explore />} />
        </Routes>
      </AuthProvider>
    </MemoryRouter>,
  );
}

function mockFetch(overrides: Record<string, Response | (() => Response)> = {}) {
  const defaults: Record<string, () => Response> = {
    "/api/auth/me": () =>
      new Response(
        JSON.stringify({ spotify_id: "anon-abc123", display_name: "Explorer" }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    "/api/venues": () =>
      new Response(JSON.stringify({ venues: [], count: 0 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    "/api/vibes/top?limit=10": () =>
      new Response(
        JSON.stringify({
          vibes: [
            { tag: "rock", prevalence: 1.0 },
            { tag: "indie", prevalence: 0.85 },
          ],
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    "/api/vibes/top?limit=500": () =>
      new Response(JSON.stringify({ vibes: [] }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
  };

  const responses: Record<string, () => Response> = {};
  for (const [url, val] of Object.entries(defaults)) {
    responses[url] = val;
  }
  for (const [url, val] of Object.entries(overrides)) {
    responses[url] = typeof val === "function" ? val : () => val;
  }

  return vi.spyOn(globalThis, "fetch").mockImplementation((input) => {
    const url =
      typeof input === "string" ? input : input instanceof URL ? input.href : (input as Request).url;
    const factory = responses[url];
    if (factory) return Promise.resolve(factory());
    if (url.startsWith("/api/vibes/related")) {
      return Promise.resolve(
        new Response(JSON.stringify({ tag: "rock", related: [] }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );
    }
    return Promise.resolve(new Response("not found", { status: 404 }));
  });
}

describe("Explore", () => {
  it("renders graph and map when already authenticated", async () => {
    mockFetch();
    renderExplore();
    // Should show the anonymous TopBar with Connect Spotify.
    await waitFor(() => {
      expect(screen.getByText("Vibe Seeker")).toBeInTheDocument();
    });
    expect(screen.getByText("Connect Spotify")).toBeInTheDocument();
    expect(screen.getByTestId("map")).toBeInTheDocument();
  });

  it("loads top vibes into graph", async () => {
    mockFetch();
    renderExplore();
    await waitFor(
      () => {
        expect(screen.getByText("rock")).toBeInTheDocument();
      },
      { timeout: 3000 },
    );
    expect(screen.getByText("indie")).toBeInTheDocument();
  });

  it("shows captcha screen when not authenticated", async () => {
    // Mock /api/auth/me to return 401 — captcha should render.
    // The Turnstile script won't actually load in tests, so we just verify the container renders.
    mockFetch({
      "/api/auth/me": new Response("unauthorized", { status: 401 }),
    });
    renderExplore();
    await waitFor(() => {
      expect(screen.getByText("Vibe Seeker")).toBeInTheDocument();
    });
    expect(screen.getByText("Discover venues that match your vibe.")).toBeInTheDocument();
  });

  it("renders search input", async () => {
    mockFetch();
    renderExplore();
    await waitFor(() => {
      expect(screen.getByPlaceholderText("Search vibes...")).toBeInTheDocument();
    });
  });

  it("renders graph controls", async () => {
    mockFetch();
    renderExplore();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^All$/ })).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: /^None$/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Reset$/ })).toBeInTheDocument();
  });
});
