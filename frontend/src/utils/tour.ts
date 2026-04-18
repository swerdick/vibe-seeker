const TOUR_COMPLETE_KEY = "vibe-seeker-tour-complete";

export function shouldAutoStartTour(): boolean {
  try {
    return !localStorage.getItem(TOUR_COMPLETE_KEY);
  } catch {
    return false;
  }
}

export function markTourComplete() {
  try {
    localStorage.setItem(TOUR_COMPLETE_KEY, "true");
  } catch {
    // Storage unavailable (e.g. private browsing) — tour will re-show next visit.
  }
}

export function resetTourComplete() {
  try {
    localStorage.removeItem(TOUR_COMPLETE_KEY);
  } catch {
    // Storage unavailable.
  }
}
