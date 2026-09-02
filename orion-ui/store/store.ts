"use client";

import { configureStore } from "@reduxjs/toolkit";
import dagReducer from "./slices/dagSlice";

export const store = configureStore({
  reducer: { dagReducer },
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
