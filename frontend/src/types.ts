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
