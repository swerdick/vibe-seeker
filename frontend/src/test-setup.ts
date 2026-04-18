import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";

// Sync controls are gated on VITE_SHOW_SYNC_CONTROLS === "true" in prod, but
// tests cover the full authenticated dev UX, including those buttons.
vi.stubEnv("VITE_SHOW_SYNC_CONTROLS", "true");

vi.mock("react-map-gl/maplibre", () => import("./__mocks__/react-map-gl-maplibre"));
vi.mock("maplibre-gl/dist/maplibre-gl.css", () => ({}));
vi.mock("react-joyride", () => ({
  Joyride: () => null,
  STATUS: { FINISHED: "finished", SKIPPED: "skipped" },
}));

// ResizeObserver is not available in jsdom.
if (typeof globalThis.ResizeObserver === "undefined") {
  globalThis.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof globalThis.ResizeObserver;
}

afterEach(() => {
  cleanup();
});
