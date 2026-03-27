import { useState, useRef, useCallback, useEffect } from "react";
import type { VibeNode, VibeEdge } from "../types";
import { fetchTopVibes } from "../utils/api";

interface VibeGraphProps {
  nodes: VibeNode[];
  edges: VibeEdge[];
  newNodeIds: Set<string>;
  selectedCount: number;
  totalCount: number;
  onToggleNode: (id: string) => void;
  onAddNode: (tag: string, prevalence?: number) => void;
  onReset: () => void;
  onSelectAll: () => void;
  onSelectNone: () => void;
}

interface SearchResult {
  tag: string;
  prevalence: number;
}

export default function VibeGraph({
  nodes,
  edges,
  newNodeIds,
  selectedCount,
  totalCount,
  onToggleNode,
  onAddNode,
  onReset,
  onSelectAll,
  onSelectNone,
}: VibeGraphProps) {
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
  const [showDropdown, setShowDropdown] = useState(false);
  const allVibesRef = useRef<SearchResult[]>([]);
  const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const containerRef = useRef<HTMLDivElement>(null);
  const [dimensions, setDimensions] = useState({ width: 400, height: 500 });

  // Pan/zoom state stored in refs for native event handlers, mirrored to state for rendering.
  const [transform, setTransform] = useState({ x: 0, y: 0, scale: 1 });
  const transformRef = useRef({ x: 0, y: 0, scale: 1 });
  const panningRef = useRef(false);
  const panStartRef = useRef({ x: 0, y: 0 });

  // Track container dimensions for SVG sizing.
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (entry) {
        setDimensions({
          width: entry.contentRect.width,
          height: entry.contentRect.height,
        });
      }
    });

    observer.observe(container);
    return () => observer.disconnect();
  }, []);

  // Attach pan/zoom via native DOM events (not React synthetic) so we can
  // use { passive: false } for wheel and get reliable mouse tracking.
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;

    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      const scaleFactor = e.deltaY > 0 ? 0.92 : 1.08;
      const prev = transformRef.current;
      const newScale = Math.min(Math.max(prev.scale * scaleFactor, 0.3), 3);
      const rect = el.getBoundingClientRect();
      const mx = e.clientX - rect.left;
      const my = e.clientY - rect.top;
      const effectiveScaleFactor = newScale / prev.scale;
      const dx = (mx - prev.x) * (1 - effectiveScaleFactor);
      const dy = (my - prev.y) * (1 - effectiveScaleFactor);
      const next = { x: prev.x + dx, y: prev.y + dy, scale: newScale };
      transformRef.current = next;
      setTransform(next);
    };

    const onMouseDown = (e: MouseEvent) => {
      if ((e.target as Element).closest(".vibe-node-group")) return;
      panningRef.current = true;
      const prev = transformRef.current;
      panStartRef.current = { x: e.clientX - prev.x, y: e.clientY - prev.y };
      el.style.cursor = "grabbing";
    };

    const onMouseMove = (e: MouseEvent) => {
      if (!panningRef.current) return;
      const next = {
        ...transformRef.current,
        x: e.clientX - panStartRef.current.x,
        y: e.clientY - panStartRef.current.y,
      };
      transformRef.current = next;
      setTransform(next);
    };

    const onMouseUp = () => {
      if (panningRef.current) {
        panningRef.current = false;
        el.style.cursor = "grab";
      }
    };

    el.addEventListener("wheel", onWheel, { passive: false });
    el.addEventListener("mousedown", onMouseDown);
    window.addEventListener("mousemove", onMouseMove);
    window.addEventListener("mouseup", onMouseUp);

    return () => {
      el.removeEventListener("wheel", onWheel);
      el.removeEventListener("mousedown", onMouseDown);
      window.removeEventListener("mousemove", onMouseMove);
      window.removeEventListener("mouseup", onMouseUp);
    };
  }, []);

  // Load all vibes for search autocomplete (lazy, once).
  const loadAllVibes = useCallback(async () => {
    if (allVibesRef.current.length > 0) return;
    try {
      const vibes = await fetchTopVibes(500);
      allVibesRef.current = vibes.map((v) => ({
        tag: v.tag,
        prevalence: v.prevalence,
      }));
    } catch {
      // Silently fail.
    }
  }, []);

  // Filter vibes list based on query, showing top results even when empty.
  const filterVibes = useCallback(
    (query: string) => {
      const nodeIds = new Set(nodes.map((n) => n.id));
      if (!query.trim()) {
        // Show top available vibes when query is empty.
        return allVibesRef.current
          .filter((v) => !nodeIds.has(v.tag))
          .slice(0, 8);
      }
      const lower = query.toLowerCase();
      return allVibesRef.current
        .filter((v) => v.tag.toLowerCase().includes(lower) && !nodeIds.has(v.tag))
        .slice(0, 8);
    },
    [nodes],
  );

  // Typeahead search — shows results on typing and on focus.
  const handleSearchChange = useCallback(
    (query: string) => {
      setSearchQuery(query);
      if (searchTimeoutRef.current) clearTimeout(searchTimeoutRef.current);

      searchTimeoutRef.current = setTimeout(async () => {
        await loadAllVibes();
        const matches = filterVibes(query);
        setSearchResults(matches);
        setShowDropdown(matches.length > 0);
      }, 100);
    },
    [loadAllVibes, filterVibes],
  );

  const handleSearchFocus = useCallback(async () => {
    await loadAllVibes();
    const matches = filterVibes(searchQuery);
    setSearchResults(matches);
    setShowDropdown(matches.length > 0);
  }, [loadAllVibes, filterVibes, searchQuery]);

  const handleSearchSelect = useCallback(
    (tag: string, prevalence?: number) => {
      onAddNode(tag, prevalence);
      setSearchQuery("");
      setSearchResults([]);
      setShowDropdown(false);
    },
    [onAddNode],
  );

  // Count edges per node to dim edges in dense areas.
  const edgeCounts = new Map<string, number>();
  for (const edge of edges) {
    edgeCounts.set(edge.source, (edgeCounts.get(edge.source) || 0) + 1);
    edgeCounts.set(edge.target, (edgeCounts.get(edge.target) || 0) + 1);
  }

  const edgeOpacity = (edge: VibeEdge) => {
    const sourceCount = edgeCounts.get(edge.source) || 1;
    const targetCount = edgeCounts.get(edge.target) || 1;
    const maxCount = Math.max(sourceCount, targetCount);
    // 1-4 edges: full visibility (~0.6)
    // 5+: drops off steeply — 5: 0.35, 8: 0.15, 12+: 0.06
    if (maxCount <= 4) return 0.6;
    return Math.max(0.06, 0.6 * Math.pow(0.55, maxCount - 4));
  };

  // Resolve edge endpoints to pixel coordinates.
  const resolveEdgeCoords = (edge: VibeEdge) => {
    const sourceNode = nodes.find((n) => n.id === edge.source);
    const targetNode = nodes.find((n) => n.id === edge.target);
    if (sourceNode?.x == null || sourceNode?.y == null || targetNode?.x == null || targetNode?.y == null) {
      return null;
    }
    return {
      x1: sourceNode.x,
      y1: sourceNode.y,
      x2: targetNode.x,
      y2: targetNode.y,
    };
  };

  const nodeRadius = (prevalence: number) => 12 + prevalence * 12;

  return (
    <div className="vibe-graph">
      <h2>Vibes</h2>

      {/* Search */}
      <div className="vibe-graph-search-container">
        <input
          type="text"
          className="vibe-graph-search"
          placeholder="Search vibes..."
          value={searchQuery}
          onChange={(e) => handleSearchChange(e.target.value)}
          onFocus={handleSearchFocus}
          onBlur={() => {
            setTimeout(() => setShowDropdown(false), 150);
          }}
        />
        {showDropdown && (
          <ul className="vibe-graph-dropdown">
            {searchResults.map((r) => (
              <li
                key={r.tag}
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => handleSearchSelect(r.tag, r.prevalence)}
              >
                {r.tag}
              </li>
            ))}
          </ul>
        )}
      </div>

      {/* Controls */}
      <div className="genre-controls">
        <button className="genre-control-btn" onClick={onSelectAll}>
          All
        </button>
        <button className="genre-control-btn" onClick={onSelectNone}>
          None
        </button>
        <button className="genre-control-btn" onClick={onReset}>
          Reset
        </button>
        <span className="genre-count-label">
          {selectedCount}/{totalCount}
        </span>
      </div>

      {/* Graph SVG with pan/zoom */}
      <div className="vibe-graph-canvas" ref={containerRef} style={{ cursor: "grab" }}>
        <svg
          width={dimensions.width}
          height={dimensions.height}
          viewBox={`0 0 ${dimensions.width} ${dimensions.height}`}
        >
          <g transform={`translate(${transform.x},${transform.y}) scale(${transform.scale})`}>
            {/* Edges */}
            {edges.map((edge) => {
              const coords = resolveEdgeCoords(edge);
              if (!coords) return null;
              return (
                <line
                  key={`${edge.source}-${edge.target}`}
                  className="vibe-edge"
                  x1={coords.x1}
                  y1={coords.y1}
                  x2={coords.x2}
                  y2={coords.y2}
                  opacity={edgeOpacity(edge)}
                />
              );
            })}

            {/* Nodes */}
            {nodes.map((node) => {
              if (node.x == null || node.y == null) return null;
              const r = nodeRadius(node.prevalence);
              const isNew = newNodeIds.has(node.id);
              return (
                <g
                  key={node.id}
                  className={`vibe-node-group ${isNew ? "vibe-node-entering" : ""}`}
                  transform={`translate(${node.x},${node.y})`}
                  onClick={() => onToggleNode(node.id)}
                  style={{ cursor: "pointer" }}
                >
                  {/* Main circle */}
                  <circle
                    className={`vibe-node ${node.active ? "vibe-node-active" : "vibe-node-inactive"}`}
                    r={r}
                  />
                  {/* Label */}
                  <text
                    className="vibe-node-label"
                    dy={r + 14}
                    textAnchor="middle"
                  >
                    {node.id}
                  </text>
                </g>
              );
            })}
          </g>
        </svg>
      </div>
    </div>
  );
}
