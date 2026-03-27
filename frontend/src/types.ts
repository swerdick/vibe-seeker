export interface User {
  spotify_id: string;
  display_name: string;
}

export interface ShowSummary {
  name: string;
  date: string;
  price_min: number;
  price_max: number;
  url: string;
}

export interface VibeNode {
  id: string; // tag name
  prevalence: number; // 0-1, controls node size
  active: boolean; // selected for matching
  expanded: boolean; // relationships loaded
  x?: number;
  y?: number;
  vx?: number;
  vy?: number;
}

export interface VibeEdge {
  source: string;
  target: string;
  strength: number; // 0-1
}

export interface VenueData {
  ID: string;
  Name: string;
  Latitude: number;
  Longitude: number;
  Address: string;
  City: string;
  State: string;
  ShowsTracked: number;
  shows: ShowSummary[] | null;
  vibes: Record<string, number> | null;
}
