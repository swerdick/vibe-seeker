import { useState, useRef, useCallback, useEffect } from "react";
import {
  forceSimulation,
  forceLink,
  forceManyBody,
  forceCenter,
  forceCollide,
  type Simulation,
  type SimulationNodeDatum,
  type SimulationLinkDatum,
} from "d3-force";
import type { VibeNode, VibeEdge } from "../types";
import { fetchTopVibes, fetchRelatedVibes } from "../utils/api";

type SimNode = VibeNode & SimulationNodeDatum;
type SimLink = SimulationLinkDatum<SimNode> & { strength: number };

interface UseVibeGraphResult {
  nodes: VibeNode[];
  edges: VibeEdge[];
  selectedTags: Set<string>;
  newNodeIds: Set<string>;
  toggleNode: (id: string) => void;
  addNode: (tag: string, prevalence?: number) => void;
  reset: () => void;
  selectAll: () => void;
  selectNone: () => void;
}

const MAX_NODES = 50;

function buildInitialNodes(vibes: Record<string, number>): SimNode[] {
  const sorted = Object.entries(vibes).sort(([, a], [, b]) => b - a);
  const top = sorted.slice(0, 10);
  const maxWeight = top[0]?.[1] ?? 1;
  return top.map(([tag, weight]) => ({
    id: tag,
    prevalence: weight / maxWeight,
    active: true,
    expanded: false,
  }));
}

// Mutate a node's properties in the d3-managed array.
// Extracted as a plain function so the React linter doesn't flag ref mutation.
function setNodeProp(nodes: SimNode[], id: string, props: Partial<SimNode>) {
  const node = nodes.find((n) => n.id === id);
  if (node) Object.assign(node, props);
}

function setAllNodeProp(nodes: SimNode[], props: Partial<SimNode>) {
  for (const n of nodes) Object.assign(n, props);
}

export function useVibeGraph(
  initialVibes?: Record<string, number> | null,
  ready = true,
): UseVibeGraphResult {
  const [nodes, setNodes] = useState<SimNode[]>([]);
  const [links, setLinks] = useState<SimLink[]>([]);
  const [selectedTags, setSelectedTags] = useState<Set<string>>(new Set());
  const [newNodeIds, setNewNodeIds] = useState<Set<string>>(new Set());
  const [resetKey, setResetKey] = useState(0);
  const simulationRef = useRef<Simulation<SimNode, SimLink> | null>(null);

  // d3 mutates these in place. Refs so toggleNode/expand can work on
  // the same objects d3 holds, preventing tick from overwriting changes.
  const nodesRef = useRef<SimNode[]>([]);
  const linksRef = useRef<SimLink[]>([]);

  const edges: VibeEdge[] = links.map((l) => ({
    source: typeof l.source === "object" ? (l.source as SimNode).id : String(l.source),
    target: typeof l.target === "object" ? (l.target as SimNode).id : String(l.target),
    strength: l.strength,
  }));

  const runSimulation = useCallback(
    (nextNodes: SimNode[], nextLinks: SimLink[], reheat = false, width = 400, height = 500) => {
      if (simulationRef.current) {
        simulationRef.current.stop();
      }

      nodesRef.current = nextNodes;
      linksRef.current = nextLinks;

      const sim = forceSimulation<SimNode>(nextNodes)
        .force(
          "link",
          forceLink<SimNode, SimLink>(nextLinks)
            .id((d) => d.id)
            .distance(120)
            .strength(0.2),
        )
        .force("charge", forceManyBody().strength(-200))
        .force("center", forceCenter(width / 2, height / 2).strength(0.03))
        .force("collide", forceCollide<SimNode>().radius(50))
        .alphaDecay(0.02)
        .velocityDecay(0.7);

      if (reheat) {
        sim.alpha(0.08);
      }

      sim.on("tick", () => {
        setNodes([...nextNodes]);
      });

      simulationRef.current = sim;
    },
    [],
  );

  // Expand a node — works directly on ref arrays to avoid d3 tick race.
  const expandRef = useRef<((id: string) => Promise<void>) | undefined>(undefined);
  useEffect(() => {
    expandRef.current = async (id: string) => {
    try {
      const related = await fetchRelatedVibes(id);
      const currentNodes = nodesRef.current;
      const currentLinks = linksRef.current;

      const existing = new Set(currentNodes.map((n) => n.id));
      const newNodes: SimNode[] = [];

      for (const r of related) {
        if (!existing.has(r.tag) && currentNodes.length + newNodes.length < MAX_NODES) {
          const parent = currentNodes.find((n) => n.id === id);
          newNodes.push({
            id: r.tag,
            prevalence: r.strength * (parent?.prevalence ?? 0.5),
            active: false,
            expanded: false,
            x: parent?.x != null ? parent.x + (Math.random() - 0.5) * 40 : undefined,
            y: parent?.y != null ? parent.y + (Math.random() - 0.5) * 40 : undefined,
          });
        }
      }

      setNodeProp(currentNodes, id, { expanded: true });
      const allNodes = [...currentNodes, ...newNodes];

      const newLinks: SimLink[] = related
        .filter((r) => allNodes.some((n) => n.id === r.tag))
        .map((r) => ({ source: id, target: r.tag, strength: r.strength }));

      const existingPairs = new Set(
        currentLinks.map((l) => {
          const s = typeof l.source === "object" ? (l.source as SimNode).id : l.source;
          const t = typeof l.target === "object" ? (l.target as SimNode).id : l.target;
          return `${s}-${t}`;
        }),
      );
      const deduped = newLinks.filter(
        (l) => !existingPairs.has(`${l.source}-${l.target}`),
      );

      const allLinks = [...currentLinks, ...deduped];
      setNewNodeIds(new Set(newNodes.map((n) => n.id)));
      setLinks(allLinks);
      runSimulation(allNodes, allLinks, true);
      setNodes([...allNodes]);
    } catch {
      // Failed to expand.
    }
  };
  }, [runSimulation]);

  const expandNode = useCallback((id: string) => {
    expandRef.current?.(id);
  }, []);

  // Clear "new" flags after entrance animation.
  useEffect(() => {
    if (newNodeIds.size === 0) return;
    const timer = setTimeout(() => setNewNodeIds(new Set()), 600);
    return () => clearTimeout(timer);
  }, [newNodeIds]);

  // Initialize the graph.
  useEffect(() => {
    if (!ready) return;
    let cancelled = false;

    const initialize = (initNodes: SimNode[], active: boolean) => {
      if (cancelled) return;
      nodesRef.current = initNodes;
      linksRef.current = [];
      setNodes(initNodes);
      setSelectedTags(active ? new Set(initNodes.map((n) => n.id)) : new Set());
      setNewNodeIds(new Set(initNodes.map((n) => n.id)));
      setLinks([]);
      runSimulation(initNodes, []);

      // Auto-expand initial active nodes to show related vibes with links.
      if (active) {
        const toExpand = initNodes.slice(0, 5);
        let delay = 200;
        for (const node of toExpand) {
          setTimeout(() => {
            if (!cancelled) expandRef.current?.(node.id);
          }, delay);
          delay += 300;
        }
      }
    };

    if (initialVibes && Object.keys(initialVibes).length > 0) {
      Promise.resolve().then(() => initialize(buildInitialNodes(initialVibes), true));
    } else {
      fetchTopVibes(10)
        .then((vibes) => {
          const initNodes: SimNode[] = vibes.map((v) => ({
            id: v.tag,
            prevalence: v.prevalence,
            active: false,
            expanded: false,
          }));
          initialize(initNodes, false);
        })
        .catch(() => {});
    }

    return () => {
      cancelled = true;
    };
  }, [resetKey, runSimulation, initialVibes, ready]);

  // Toggle active — mutate d3 node directly so ticks don't overwrite.
  const toggleNode = useCallback(
    (id: string) => {
      const node = nodesRef.current.find((n) => n.id === id);
      if (!node) return;

      const nowActive = !node.active;
      setNodeProp(nodesRef.current, id, { active: nowActive });

      setSelectedTags((prev) => {
        const next = new Set(prev);
        if (nowActive) next.add(id);
        else next.delete(id);
        return next;
      });

      if (nowActive && !node.expanded) {
        expandNode(id);
      }

      setNodes([...nodesRef.current]);
    },
    [expandNode],
  );

  const addNode = useCallback(
    (tag: string, prevalence = 0.5) => {
      const existing = nodesRef.current.find((n) => n.id === tag);
      if (existing) {
        setNodeProp(nodesRef.current, tag, { active: true });
        setSelectedTags((s) => new Set(s).add(tag));
        setNodes([...nodesRef.current]);
        return;
      }

      if (nodesRef.current.length >= MAX_NODES) return;

      const newNode: SimNode = {
        id: tag,
        prevalence,
        active: true,
        expanded: false,
      };

      const allNodes = [...nodesRef.current, newNode];
      setSelectedTags((s) => new Set(s).add(tag));
      setNewNodeIds(new Set([tag]));
      runSimulation(allNodes, linksRef.current, true);
      setNodes([...allNodes]);
      expandNode(tag);
    },
    [runSimulation, expandNode],
  );

  const reset = useCallback(() => {
    if (simulationRef.current) {
      simulationRef.current.stop();
      simulationRef.current = null;
    }
    nodesRef.current = [];
    linksRef.current = [];
    setResetKey((k) => k + 1);
  }, []);

  const selectAll = useCallback(() => {
    setAllNodeProp(nodesRef.current, { active: true });
    setSelectedTags(new Set(nodesRef.current.map((n) => n.id)));
    setNodes([...nodesRef.current]);
  }, []);

  const selectNone = useCallback(() => {
    setAllNodeProp(nodesRef.current, { active: false });
    setSelectedTags(new Set());
    setNodes([...nodesRef.current]);
  }, []);

  useEffect(() => {
    return () => {
      if (simulationRef.current) {
        simulationRef.current.stop();
      }
    };
  }, []);

  return {
    nodes,
    edges,
    selectedTags,
    newNodeIds,
    toggleNode,
    addNode,
    reset,
    selectAll,
    selectNone,
  };
}
