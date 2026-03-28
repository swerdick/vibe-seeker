import { describe, it, expect, vi, beforeEach } from "vitest";

describe("initTelemetry", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.unstubAllEnvs();
  });

  it("does not throw when disabled", async () => {
    vi.stubEnv("VITE_OTEL_ENABLED", "");
    const { initTelemetry } = await import("./otel.ts");
    expect(() => initTelemetry()).not.toThrow();
  });

  it("does not throw when enabled", async () => {
    vi.stubEnv("VITE_OTEL_ENABLED", "true");
    const { initTelemetry } = await import("./otel.ts");
    expect(() => initTelemetry()).not.toThrow();
  });
});
