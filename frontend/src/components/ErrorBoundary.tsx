import { Component } from "react";
import type { ErrorInfo, ReactNode } from "react";
import { trace } from "@opentelemetry/api";

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false };

  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    const tracer = trace.getTracer("react-error-boundary");
    const span = tracer.startSpan("browser.react_error", {
      attributes: {
        "error.type": "react_render",
        "error.message": error.message,
        "error.stack": error.stack || "",
        "error.component_stack": info.componentStack || "",
      },
    });
    span.end();
  }

  render(): ReactNode {
    if (this.state.hasError) {
      return (
        <div className="page">
          <h1>Something went wrong</h1>
          <p>An unexpected error occurred.</p>
          <button
            className="button"
            onClick={() => this.setState({ hasError: false })}
          >
            Try again
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
