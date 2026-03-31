import { AlertTriangle, RotateCcw } from "lucide-react";
import { Component, type ErrorInfo, type ReactNode } from "react";

type Props = { children: ReactNode };
type State = { error: Error | null };

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("[ErrorBoundary]", error, info.componentStack);
  }

  render() {
    if (!this.state.error) return this.props.children;

    return (
      <div className="flex min-h-[50vh] items-center justify-center p-8">
        <div className="flex max-w-md flex-col items-center gap-5 text-center">
          <div className="grid h-16 w-16 place-items-center rounded-2xl bg-status-danger/10">
            <AlertTriangle size={28} strokeWidth={1.6} className="text-status-danger" />
          </div>
          <h2 className="text-[18px] font-bold text-txt-primary">Something went wrong</h2>
          {this.state.error.message ? (
            <p className="max-w-[520px] break-words text-[12px] text-txt-secondary">{this.state.error.message}</p>
          ) : null}
          <button
            type="button"
            onClick={() => this.setState({ error: null })}
            className="inline-flex items-center gap-2 rounded-xl bg-accent/15 px-5 py-2.5 text-[14px] font-semibold text-accent transition-colors hover:bg-accent/25"
          >
            <RotateCcw size={16} strokeWidth={1.8} />
            Retry
          </button>
        </div>
      </div>
    );
  }
}
