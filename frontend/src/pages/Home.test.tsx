import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../contexts/AuthContext";
import ProtectedRoute from "../components/ProtectedRoute";
import Home from "./Home";

beforeEach(() => {
  vi.restoreAllMocks();
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderHome() {
  return render(
    <MemoryRouter initialEntries={["/home"]}>
      <AuthProvider>
        <Routes>
          <Route path="/" element={<p>login page</p>} />
          <Route element={<ProtectedRoute />}>
            <Route path="/home" element={<Home />} />
          </Route>
        </Routes>
      </AuthProvider>
    </MemoryRouter>,
  );
}

// URL-based fetch mock — avoids fragile positional mocking.
function mockFetch(overrides: Record<string, Response | (() => Response)> = {}) {
  const defaults: Record<string, () => Response> = {
    "/api/auth/me": () =>
      new Response(
        JSON.stringify({ spotify_id: "spotify123", display_name: "Test User" }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    "/api/vibe": () =>
      new Response(JSON.stringify({ vibes: {}, vibe_count: 0 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    "/api/venues": () =>
      new Response(JSON.stringify({ venues: [], count: 0 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    "/api/vibes/top?limit=10": () =>
      new Response(JSON.stringify({ vibes: [] }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
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

  const mock = vi.spyOn(globalThis, "fetch").mockImplementation((input) => {
    const url =
      typeof input === "string" ? input : input instanceof URL ? input.href : (input as Request).url;
    // Exact match first, then prefix match for parameterized URLs.
    const factory = responses[url];
    if (factory) return Promise.resolve(factory());
    // Handle /api/vibes/related?tag=... dynamically.
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

  return mock;
}

describe("Home", () => {
  it("renders the home page when authenticated", async () => {
    mockFetch();
    renderHome();
    await waitFor(() => {
      expect(screen.getByText(/Hello, Test User/)).toBeInTheDocument();
    });
    expect(
      screen.getByRole("button", { name: /sync vibe/i }),
    ).toBeInTheDocument();
  });

  it("redirects to login when not authenticated", async () => {
    mockFetch({
      "/api/auth/me": new Response("unauthorized", { status: 401 }),
    });
    renderHome();
    await waitFor(() => {
      expect(screen.getByText("login page")).toBeInTheDocument();
    });
  });

  it("calls logout endpoint and redirects on logout", async () => {
    const fetchMock = mockFetch({
      "/api/auth/logout": new Response(null, { status: 204 }),
    });
    renderHome();
    await waitFor(() => {
      expect(screen.getByText(/Hello, Test User/)).toBeInTheDocument();
    });

    await userEvent.click(screen.getByRole("button", { name: /log out/i }));
    await waitFor(() => {
      expect(screen.getByText("login page")).toBeInTheDocument();
    });

    expect(fetchMock).toHaveBeenCalledWith("/api/auth/logout", {
      method: "POST",
      credentials: "include",
    });
  });

  it("renders Sync Vibe button after auth", async () => {
    mockFetch();
    renderHome();
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /sync vibe/i }),
      ).toBeInTheDocument();
    });
  });

  it("displays vibes in graph after loading", async () => {
    mockFetch({
      "/api/vibe": new Response(
        JSON.stringify({
          vibes: { rock: 1.0, indie: 0.7, "dream pop": 0.3 },
          vibe_count: 3,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    });
    renderHome();
    // Graph renders node labels as SVG text; d3-force simulation may take a tick.
    await waitFor(
      () => {
        expect(screen.getByText("rock")).toBeInTheDocument();
      },
      { timeout: 3000 },
    );
    expect(screen.getByText("indie")).toBeInTheDocument();
    expect(screen.getByText("dream pop")).toBeInTheDocument();
  });

  it("calls sync endpoint and refreshes vibes on click", async () => {
    let vibeCalls = 0;
    mockFetch({
      "/api/vibe": () => {
        vibeCalls++;
        const vibeData = vibeCalls > 1 ? { rock: 1.0, indie: 0.5 } : {};
        return new Response(
          JSON.stringify({ vibes: vibeData, vibe_count: Object.keys(vibeData).length }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      },
      "/api/vibe/sync": new Response(
        JSON.stringify({ synced: true, vibe_count: 2 }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    });

    renderHome();
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /sync vibe/i }),
      ).toBeInTheDocument();
    });

    await userEvent.click(
      screen.getByRole("button", { name: /sync vibe/i }),
    );

    await waitFor(
      () => {
        expect(screen.getByText("rock")).toBeInTheDocument();
      },
      { timeout: 3000 },
    );
  });

  it("shows error when sync fails", async () => {
    mockFetch({
      "/api/vibe/sync": new Response("error", { status: 502 }),
    });
    renderHome();
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /sync vibe/i }),
      ).toBeInTheDocument();
    });

    await userEvent.click(
      screen.getByRole("button", { name: /sync vibe/i }),
    );

    await waitFor(() => {
      expect(
        screen.getByText(/failed to sync vibe/i),
      ).toBeInTheDocument();
    });
  });

  it("handles empty vibe gracefully", async () => {
    mockFetch();
    renderHome();
    await waitFor(() => {
      expect(screen.getByText(/Hello, Test User/)).toBeInTheDocument();
    });
    expect(screen.queryByText("Your Top Genres")).not.toBeInTheDocument();
  });

  it("renders Sync Venues button after auth", async () => {
    mockFetch();
    renderHome();
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /sync venues/i }),
      ).toBeInTheDocument();
    });
  });

  it("calls venue sync endpoint and shows count", async () => {
    mockFetch({
      "/api/venues/sync": new Response(
        JSON.stringify({ synced: true, venues_count: 42, shows_count: 100 }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    });
    renderHome();
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /sync venues/i }),
      ).toBeInTheDocument();
    });

    await userEvent.click(
      screen.getByRole("button", { name: /sync venues/i }),
    );

    await waitFor(() => {
      expect(screen.getByText(/42 venues/i)).toBeInTheDocument();
    });
  });

  it("shows error when venue sync fails", async () => {
    mockFetch({
      "/api/venues/sync": new Response("error", { status: 502 }),
    });
    renderHome();
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /sync venues/i }),
      ).toBeInTheDocument();
    });

    await userEvent.click(
      screen.getByRole("button", { name: /sync venues/i }),
    );

    await waitFor(() => {
      expect(
        screen.getByText(/failed to sync venues/i),
      ).toBeInTheDocument();
    });
  });

  it("renders map container", async () => {
    mockFetch();
    renderHome();
    await waitFor(() => {
      expect(screen.getByTestId("map")).toBeInTheDocument();
    });
  });
});
