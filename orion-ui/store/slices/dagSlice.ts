"use client";

import { createSlice } from "@reduxjs/toolkit";

const dagSlice = createSlice({
  name: "dag",
  initialState: {
    nodes: [],
    edges: [],
  },
  reducers: {
    setNodes: (state, action) => {
      state.nodes = action.payload;
    },
    setEdges: (state, action) => {
      state.edges = action.payload;
    },
  },
});

export const { setNodes, setEdges } = dagSlice.actions;
export default dagSlice.reducer;
