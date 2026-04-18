const TOUR_COMPLETE_KEY = "vibe-seeker-tour-complete";

export function shouldAutoStartTour(): boolean {
  return !localStorage.getItem(TOUR_COMPLETE_KEY);
}

export function markTourComplete() {
  localStorage.setItem(TOUR_COMPLETE_KEY, "true");
}

export function resetTourComplete() {
  localStorage.removeItem(TOUR_COMPLETE_KEY);
}
