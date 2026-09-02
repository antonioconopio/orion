"use client";

import { ThemeProvider } from "next-themes";
import { Provider } from "react-redux";
import { SidebarProvider } from "@/components/ui/sidebar";
import { store } from "@/store/store";

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <Provider store={store}>
      <ThemeProvider attribute="class">
        <SidebarProvider>{children}</SidebarProvider>
      </ThemeProvider>
    </Provider>
  );
}
