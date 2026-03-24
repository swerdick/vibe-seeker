// Continuous gradient from gray (0%) through warm tones to Spotify green (100%).
export function getMatchColor(score: number): string {
  if (score <= 0) return "hsl(0, 0%, 40%)";
  const s = Math.min(score, 1);
  const hue = s * 142;
  const sat = s * 76;
  const lit = 40 + s * 14;
  return `hsl(${Math.round(hue)}, ${Math.round(sat)}%, ${Math.round(lit)}%)`;
}

// Cosine similarity between two tag-weight vectors.
export function cosineSimilarity(
  a: Record<string, number>,
  b: Record<string, number>,
): number {
  let dot = 0;
  let magA = 0;
  let magB = 0;

  const allKeys = new Set([...Object.keys(a), ...Object.keys(b)]);
  for (const key of allKeys) {
    const va = a[key] || 0;
    const vb = b[key] || 0;
    dot += va * vb;
    magA += va * va;
    magB += vb * vb;
  }

  if (magA === 0 || magB === 0) return 0;
  return dot / (Math.sqrt(magA) * Math.sqrt(magB));
}
