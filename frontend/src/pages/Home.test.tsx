import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
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
      <Routes>
        <Route path="/" element={<p>login page</p>} />
        <Route path="/home" element={<Home />} />
      </Routes>
    </MemoryRouter>,
  );
}

const meResponse = () =>
  new Response(
    JSON.stringify({ spotify_id: "spotify123", display_name: "Test User" }),
    { status: 200, headers: { "Content-Type": "application/json" } },
  );

const tasteResponse = (genres: Record<string, number> = {}) =>
  new Response(
    JSON.stringify({ genres, genre_count: Object.keys(genres).length }),
    { status: 200, headers: { "Content-Type": "application/json" } },
  );

describe("Home", () => {
  it("renders the home page when authenticated", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch");
    fetchMock.mockResolvedValueOnce(meResponse());
    fetchMock.mockResolvedValueOnce(tasteResponse());

    renderHome();
    await waitFor(() => {
      expect(screen.getByText(/Hello, Test User/)).toBeInTheDocument();
    });
    expect(screen.getByText(/you are logged in/i)).toBeInTheDocument();
  });

  it("redirects to login when not authenticated", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response("unauthorized", { status: 401 }),
    );

    renderHome();
    await waitFor(() => {
      expect(screen.getByText("login page")).toBeInTheDocument();
    });
  });

  it("calls logout endpoint and redirects on logout", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch");
    fetchMock.mockResolvedValueOnce(meResponse());
    fetchMock.mockResolvedValueOnce(tasteResponse());

    renderHome();
    await waitFor(() => {
      expect(screen.getByText(/Hello, Test User/)).toBeInTheDocument();
    });

    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
    await userEvent.click(screen.getByRole("button", { name: /log out/i }));

    await waitFor(() => {
      expect(screen.getByText("login page")).toBeInTheDocument();
    });

    expect(fetchMock).toHaveBeenCalledWith("/api/auth/logout", {
      method: "POST",
      credentials: "include",
    });
  });

  it("renders Sync Taste button after auth", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch");
    fetchMock.mockResolvedValueOnce(meResponse());
    fetchMock.mockResolvedValueOnce(tasteResponse());

    renderHome();
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /sync taste/i }),
      ).toBeInTheDocument();
    });
  });

  it("displays genres after loading taste", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch");
    fetchMock.mockResolvedValueOnce(meResponse());
    fetchMock.mockResolvedValueOnce(
      tasteResponse({ rock: 1.0, indie: 0.7, "dream pop": 0.3 }),
    );

    renderHome();
    await waitFor(() => {
      expect(screen.getByText("rock")).toBeInTheDocument();
    });
    expect(screen.getByText("indie")).toBeInTheDocument();
    expect(screen.getByText("dream pop")).toBeInTheDocument();
  });

  it("calls sync endpoint and refreshes genres on click", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch");
    fetchMock.mockResolvedValueOnce(meResponse());
    fetchMock.mockResolvedValueOnce(tasteResponse()); // initial empty taste
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ synced: true, genre_count: 2 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    ); // sync response
    fetchMock.mockResolvedValueOnce(
      tasteResponse({ rock: 1.0, indie: 0.5 }),
    ); // refreshed taste

    renderHome();
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /sync taste/i }),
      ).toBeInTheDocument();
    });

    await userEvent.click(
      screen.getByRole("button", { name: /sync taste/i }),
    );

    await waitFor(() => {
      expect(screen.getByText("rock")).toBeInTheDocument();
    });
  });

  it("shows error when sync fails", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch");
    fetchMock.mockResolvedValueOnce(meResponse());
    fetchMock.mockResolvedValueOnce(tasteResponse());
    fetchMock.mockResolvedValueOnce(
      new Response("error", { status: 502 }),
    ); // sync fails

    renderHome();
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /sync taste/i }),
      ).toBeInTheDocument();
    });

    await userEvent.click(
      screen.getByRole("button", { name: /sync taste/i }),
    );

    await waitFor(() => {
      expect(
        screen.getByText(/failed to sync taste/i),
      ).toBeInTheDocument();
    });
  });

  it("handles empty taste gracefully", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch");
    fetchMock.mockResolvedValueOnce(meResponse());
    fetchMock.mockResolvedValueOnce(tasteResponse({}));

    renderHome();
    await waitFor(() => {
      expect(screen.getByText(/Hello, Test User/)).toBeInTheDocument();
    });

    expect(screen.queryByText("Your Top Genres")).not.toBeInTheDocument();
  });
});
