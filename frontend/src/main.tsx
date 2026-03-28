import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import { initTelemetry } from "./utils/otel.ts";
import { ErrorBoundary } from "./components/ErrorBoundary.tsx";
import App from "./App.tsx";

initTelemetry();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </StrictMode>,
);
