import { useEffect, useRef, useState } from "react";

export type EventStreamStatus =
  | "idle"
  | "connecting"
  | "connected"
  | "disconnected"
  | "error";

export interface StreamEvent {
  type: string;
  projectId: string;
  sessionId: string;
  payload: Record<string, unknown>;
}

// DeltaPayload mirrors OpenCode's `message.part.delta` properties.
export interface DeltaPayload {
  sessionID: string;
  messageID: string;
  partID: string;
  field: string;
  delta: string;
}

// PartUpdatedPayload mirrors OpenCode's `message.part.updated` properties.
export interface PartUpdatedPayload {
  sessionID: string;
  part: {
    messageID: string;
    type: string;
    text?: string;
  };
  time?: number;
}

export function useEventStream(projectId: string | null): {
  status: EventStreamStatus;
  latestEvent: StreamEvent | null;
} {
  const [status, setStatus] = useState<EventStreamStatus>("idle");
  const [latestEvent, setLatestEvent] = useState<StreamEvent | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (!projectId) {
      setStatus("idle");
      setLatestEvent(null);
      return;
    }

    let cancelled = false;
    setStatus("connecting");

    const url = `/api/events?projectId=${encodeURIComponent(projectId)}`;
    const es = new EventSource(url);
    eventSourceRef.current = es;

    es.onopen = () => {
      if (!cancelled) {
        setStatus("connected");
      }
    };

    es.onmessage = (event: MessageEvent) => {
      if (cancelled) return;
      try {
        const parsed = JSON.parse(event.data) as StreamEvent;
        setLatestEvent(parsed);
      } catch {
        /* Ignore unparseable events */
      }
    };

    es.onerror = () => {
      if (cancelled) return;
      if (es.readyState === EventSource.CLOSED) {
        setStatus("disconnected");
      } else {
        setStatus("error");
      }
    };

    return () => {
      cancelled = true;
      es.close();
      eventSourceRef.current = null;
    };
  }, [projectId]);

  return { status, latestEvent };
}
