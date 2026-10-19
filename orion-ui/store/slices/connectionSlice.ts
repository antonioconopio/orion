"use client";

import { createSlice, type PayloadAction } from "@reduxjs/toolkit";

export type ConnectionState = "connecting" | "connected" | "disconnected";

// Whether the live-updates WebSocket is up; the header indicator reads this.
const connectionSlice = createSlice({
  name: "connection",
  initialState: "connecting" as ConnectionState,
  reducers: {
    setConnection: (_state, action: PayloadAction<ConnectionState>) => action.payload,
  },
});

export const { setConnection } = connectionSlice.actions;
export default connectionSlice.reducer;
