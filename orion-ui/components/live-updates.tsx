"use client";

import { useEffect } from "react";

import { WS_URL } from "@/lib/api/config";
import { backoffMs, parseEvent, tagsForEvent, uniqueTags, type CacheTag } from "@/lib/api/events";
import { api } from "@/store/api";
import { useAppDispatch } from "@/store/hooks";
import { setConnection } from "@/store/slices/connectionSlice";

const BATCH_MS = 150; // events that arrive together cause one refetch, not one each
const ALL_TAGS = ["DagList", "Dag", "DagDefinition", "RunList", "Run", "Instances", "Stats", "Workers"] as const;

/**
 * Holds the WebSocket to the API open and turns each event into "this data is stale" for RTK Query,
 * which refetches whatever is on screen. Renders nothing; mounted once in Providers.
 */
export function LiveUpdates() {
  const dispatch = useAppDispatch();

  useEffect(() => {
    let socket: WebSocket | undefined;
    let disposed = false;
    let attempt = 0;
    let missedEvents = false; // set when the connection drops: events may have been lost meanwhile
    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    let flushTimer: ReturnType<typeof setTimeout> | undefined;
    let pending: CacheTag[] = [];

    const flush = () => {
      flushTimer = undefined;
      const tags = uniqueTags(pending);
      pending = [];
      if (tags.length > 0) dispatch(api.util.invalidateTags(tags));
    };

    const connect = () => {
      socket = new WebSocket(WS_URL);

      socket.onopen = () => {
        attempt = 0;
        dispatch(setConnection("connected"));
        if (missedEvents) {
          dispatch(api.util.invalidateTags([...ALL_TAGS])); // catch up on anything we missed
          missedEvents = false;
        }
      };

      socket.onmessage = (message) => {
        const event = parseEvent(message.data);
        if (!event) return;
        pending.push(...tagsForEvent(event));
        flushTimer ??= setTimeout(flush, BATCH_MS);
      };

      socket.onclose = () => {
        if (disposed) return;
        missedEvents = true;
        dispatch(setConnection("disconnected"));
        retryTimer = setTimeout(connect, backoffMs(attempt++));
      };

      socket.onerror = () => socket?.close();
    };

    connect();
    return () => {
      disposed = true;
      clearTimeout(retryTimer);
      clearTimeout(flushTimer);
      socket?.close();
    };
  }, [dispatch]);

  return null;
}
