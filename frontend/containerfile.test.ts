import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const projectRoot = resolve(fileURLToPath(import.meta.url), "..");

function readProjectFile(name: string): string {
  return readFileSync(resolve(projectRoot, name), "utf-8");
}

function parseIgnoreEntries(content: string): string[] {
  return content
    .split("\n")
    .map((l) => l.trim())
    .filter((l) => l && !l.startsWith("#"));
}

describe("Containerfile build context", () => {
  it(".dockerignore excludes node_modules so host binaries don't overwrite npm ci", () => {
    const entries = parseIgnoreEntries(readProjectFile(".dockerignore"));
    expect(entries).toContain("node_modules");
  });

  it(".dockerignore excludes dist so local builds don't leak into the image", () => {
    const entries = parseIgnoreEntries(readProjectFile(".dockerignore"));
    expect(entries).toContain("dist");
  });

  it("Containerfile runs npm ci before COPY . . to install platform-native deps", () => {
    const lines = readProjectFile("Containerfile")
      .split("\n")
      .map((l) => l.trim())
      .filter((l) => l && !l.startsWith("#"));

    const npmCiIndex = lines.findIndex((l) => l.startsWith("RUN npm ci"));
    const copyAllIndex = lines.findIndex(
      (l, i) => i > npmCiIndex && l === "COPY . .",
    );

    expect(npmCiIndex).toBeGreaterThan(-1);
    expect(copyAllIndex).toBeGreaterThan(npmCiIndex);
  });
});
