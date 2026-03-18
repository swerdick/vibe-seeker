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

describe("Home", () => {
  it("renders the home page when authenticated", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          spotify_id: "spotify123",
          display_name: "Test User",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

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

    // First call: /api/auth/me
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          spotify_id: "spotify123",
          display_name: "Test User",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    renderHome();
    await waitFor(() => {
      expect(screen.getByText(/Hello, Test User/)).toBeInTheDocument();
    });

    // Second call: /api/auth/logout
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
});
