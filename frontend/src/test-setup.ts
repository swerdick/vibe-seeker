import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";

vi.mock("react-map-gl/maplibre", () => import("./__mocks__/react-map-gl-maplibre"));
vi.mock("maplibre-gl/dist/maplibre-gl.css", () => ({}));

afterEach(() => {
  cleanup();
});
