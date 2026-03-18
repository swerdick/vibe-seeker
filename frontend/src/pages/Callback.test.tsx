import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it } from "vitest";
import Callback from "./Callback";

function renderCallback() {
  return render(
    <MemoryRouter initialEntries={["/callback"]}>
      <Routes>
        <Route path="/callback" element={<Callback />} />
        <Route path="/home" element={<p>home page</p>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("Callback", () => {
  it("redirects to /home", async () => {
    renderCallback();
    await waitFor(() => {
      expect(screen.getByText("home page")).toBeInTheDocument();
    });
  });
});
