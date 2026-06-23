import { vi } from "vitest";

type EventSourceHandler = ((event: Event) => void) | null;

/** Controllable EventSource stand-in for SSE hook tests. */
export class FakeEventSource {
  static instances: FakeEventSource[] = [];

  readonly url: string;
  readyState = 0;
  onopen: EventSourceHandler = null;
  onmessage: EventSourceHandler = null;
  onerror: EventSourceHandler = null;
  closed = false;

  constructor(url: string | URL) {
    this.url = String(url);
    FakeEventSource.instances.push(this);
    queueMicrotask(() => {
      if (!this.closed) {
        this.readyState = 1;
        this.onopen?.(new Event("open"));
      }
    });
  }

  emit(data: string): void {
    this.onmessage?.(new MessageEvent("message", { data }));
  }

  emitError(): void {
    this.onerror?.(new Event("error"));
  }

  close(): void {
    this.closed = true;
    this.readyState = 2;
  }
}

let originalEventSource: typeof EventSource | undefined;

export function installFakeEventSource(): void {
  FakeEventSource.instances = [];
  originalEventSource = globalThis.EventSource;
  vi.stubGlobal("EventSource", FakeEventSource);
}

export function restoreEventSource(): void {
  if (originalEventSource !== undefined) {
    vi.stubGlobal("EventSource", originalEventSource);
    originalEventSource = undefined;
  }
  FakeEventSource.instances = [];
}
