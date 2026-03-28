import { trace } from "@opentelemetry/api";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { registerInstrumentations } from "@opentelemetry/instrumentation";
import { DocumentLoadInstrumentation } from "@opentelemetry/instrumentation-document-load";
import { FetchInstrumentation } from "@opentelemetry/instrumentation-fetch";
import { resourceFromAttributes } from "@opentelemetry/resources";
import {
  SEMRESATTRS_DEPLOYMENT_ENVIRONMENT,
  SEMRESATTRS_SERVICE_NAME,
} from "@opentelemetry/semantic-conventions";
import {
  BatchSpanProcessor,
  WebTracerProvider,
} from "@opentelemetry/sdk-trace-web";
import { onCLS, onFCP, onINP, onLCP, onTTFB } from "web-vitals";
import type { Metric } from "web-vitals";

export function initTelemetry(): void {
  if (import.meta.env.VITE_OTEL_ENABLED !== "true") return;

  const serviceName =
    import.meta.env.VITE_OTEL_SERVICE_NAME || "vibe-seeker-web";
  const environment = import.meta.env.VITE_OTEL_ENVIRONMENT || "local";

  const provider = new WebTracerProvider({
    resource: resourceFromAttributes({
      [SEMRESATTRS_SERVICE_NAME]: serviceName,
      [SEMRESATTRS_DEPLOYMENT_ENVIRONMENT]: environment,
    }),
    spanProcessors: [
      new BatchSpanProcessor(
        new OTLPTraceExporter({ url: "/api/otlp/v1/traces" }),
      ),
    ],
  });

  provider.register();

  registerInstrumentations({
    instrumentations: [
      new FetchInstrumentation({
        propagateTraceHeaderCorsUrls: [/^\/api\//],
      }),
      new DocumentLoadInstrumentation(),
    ],
  });

  setupErrorHandlers();
  setupWebVitals();
}

function setupErrorHandlers(): void {
  const tracer = trace.getTracer("error-handler");

  window.addEventListener("error", (event) => {
    const span = tracer.startSpan("browser.error", {
      attributes: {
        "error.type": "uncaught",
        "error.message": event.message,
        "error.filename": event.filename || "",
        "error.lineno": event.lineno || 0,
        "error.colno": event.colno || 0,
      },
    });
    span.end();
  });

  window.addEventListener("unhandledrejection", (event) => {
    const span = tracer.startSpan("browser.unhandledrejection", {
      attributes: {
        "error.type": "unhandled_promise_rejection",
        "error.message":
          event.reason instanceof Error
            ? event.reason.message
            : String(event.reason),
      },
    });
    span.end();
  });
}

function setupWebVitals(): void {
  const tracer = trace.getTracer("web-vitals");

  const report = (metric: Metric) => {
    const span = tracer.startSpan(`web-vital.${metric.name}`, {
      attributes: {
        "web_vital.name": metric.name,
        "web_vital.value": metric.value,
        "web_vital.rating": metric.rating,
        "web_vital.id": metric.id,
        "web_vital.navigation_type": metric.navigationType,
      },
    });
    span.end();
  };

  onCLS(report);
  onLCP(report);
  onINP(report);
  onFCP(report);
  onTTFB(report);
}
