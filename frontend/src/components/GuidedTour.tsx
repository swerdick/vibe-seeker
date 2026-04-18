import { Joyride, STATUS, type EventData } from "react-joyride";
import { markTourComplete } from "../utils/tour";

const steps = [
  {
    target: '[data-tour="vibe-panel"]',
    content: "This is your vibe profile. Select genre nodes (last.fm tags) for music that you'd like to hear, or search for specific genres in the search bar. Related genres will appear as you click!",
    skipBeacon: true,
    placement: "left" as const,
  },
  {
    target: '[data-tour="venue-map"]',
    content: "Venues will be color coded by how well the music they typically play matches the vibe you're building on the right. Click a marker for details and to see upcoming shows.",
    skipBeacon: true,
    placement: "right" as const,
  },
  {
    target: '[data-tour="sync-vibe"]',
    content: "Click here to pull your music taste from Spotify and auto-populate a vibe based on your last 6 months of played songs.",
    skipBeacon: true,
    placement: "bottom" as const,
  },
  {
    target: '[data-tour="match-slider"]',
    content: "Drag this slider to only show venues above a certain match percentage.",
    skipBeacon: true,
    placement: "bottom" as const,
  },
];

interface GuidedTourProps {
  run: boolean;
  onFinish: () => void;
}

export default function GuidedTour({ run, onFinish }: GuidedTourProps) {
  const handleEvent = (data: EventData) => {
    if (data.status === STATUS.FINISHED || data.status === STATUS.SKIPPED) {
      markTourComplete();
      onFinish();
    }
  };

  return (
    <Joyride
      steps={steps}
      run={run}
      continuous
      locale={{ last: "Done" }}
      onEvent={handleEvent}
      options={{
        backgroundColor: "#1a1a1a",
        textColor: "#fff",
        primaryColor: "#1db954",
        arrowColor: "#1a1a1a",
        overlayColor: "rgba(0, 0, 0, 0.6)",
        zIndex: 100,
        showProgress: true,
        buttons: ["back", "skip", "primary"],
      }}
    />
  );
}
