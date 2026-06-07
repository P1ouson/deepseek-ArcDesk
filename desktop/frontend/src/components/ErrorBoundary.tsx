import { Component, type ReactNode } from "react";
import { t } from "../lib/i18n";
import { reportCrash } from "../lib/crash";

type ErrorBoundaryState = { crashed: boolean; detail?: string };

export class ErrorBoundary extends Component<{ children: ReactNode }, ErrorBoundaryState> {
  state: ErrorBoundaryState = { crashed: false };

  static getDerivedStateFromError(error: unknown): ErrorBoundaryState {
    const detail =
      error instanceof Error ? error.stack ?? error.message : error != null ? String(error) : undefined;
    return { crashed: true, detail };
  }

  componentDidCatch(error: unknown, info: { componentStack?: string | null }) {
    reportCrash("react", error, info.componentStack ?? undefined);
  }

  render() {
    if (this.state.crashed) {
      return (
        <div className="error-boundary" role="alert">
          <div className="error-boundary__title">{t("errorBoundary.title")}</div>
          <p className="error-boundary__message">{t("errorBoundary.message")}</p>
          {this.state.detail ? <pre className="error-boundary__body">{this.state.detail}</pre> : null}
          <button type="button" className="btn btn--primary error-boundary__reload" onClick={() => window.location.reload()}>
            {t("errorBoundary.reload")}
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
