"use client";

import { configureStore } from "@reduxjs/toolkit";
import { api } from "./api";
import dagReducer from "./slices/dagSlice";

export const store = configureStore({
  reducer: {
    dagReducer,
    [api.reducerPath]: api.reducer,
  },
  middleware: (getDefault) => getDefault().concat(api.middleware),
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
