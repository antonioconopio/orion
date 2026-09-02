"use client";

import { usePathname, useRouter } from "next/navigation";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  SidebarTrigger,
} from "@/components/ui/sidebar";
import {
  ChevronDown,
  House,
  Network,
  Activity,
  Server,
  Settings,
  Moon,
  Sun,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "./ui/dropdown-menu";
import User2 from "./icons/user2";
import Logo from "./icons/logo";
import ThemeToggle from "./theme-toggle";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "./ui/collapsible";

import { useTheme } from "next-themes";

const NAV_ITEMS = [
  { label: "Dashboard", href: "/dashboard", icon: House },
  { label: "DAGs", href: "/dags", icon: Network },
  { label: "Runs", href: "/runs", icon: Activity },
  { label: "Workers", href: "/workers", icon: Server },
  { label: "Settings", href: "/settings", icon: Settings },
];

export function AppSidebar() {
  const router = useRouter();
  const pathname = usePathname();
  const { theme, setTheme } = useTheme();
  return (
    <Sidebar collapsible="icon" className="bg-white dark:bg-[#0a0a0f]">
      {/* HEADER */}
      <SidebarHeader>
        <div className="flex flex-row items-center justify-between group-data-[state=collapsed]:flex-col group-data-[state=collapsed]:gap-1">
          <Logo />
          <div className="flex flex-row align-baseline w-full gap-2 justify-between group-data-[state=collapsed]:hidden">
            <div className="flex flex-row items-center gap-2">
              <span className="px-2">
                <h1 className="text-sm font-semibold">ORION</h1>
                <p className="text-[0.6rem] text-gray-600">ORCHESTRATION</p>
              </span>
            </div>
          </div>
          <SidebarTrigger className="hover:cursor-pointer rounded-2xl" />
        </div>
      </SidebarHeader>

      <hr />

      {/* CONTENT */}
      <SidebarContent>
        {/* GROUPS */}
        <Collapsible defaultOpen className="group/collapsible">
          <SidebarGroup>
            <SidebarGroupLabel asChild>
              <CollapsibleTrigger>
                WORKSPACE
                <ChevronDown className="ml-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-180 hover:cursor-pointer" />
              </CollapsibleTrigger>
            </SidebarGroupLabel>
            <CollapsibleContent>
              <SidebarMenu>
                {NAV_ITEMS.map((item) => {
                  const Icon = item.icon;
                  const active =
                    pathname === item.href ||
                    pathname.startsWith(`${item.href}/`);
                  return (
                    <SidebarMenuItem key={item.href}>
                      <SidebarMenuButton
                        isActive={active}
                        className="rounded-2xl transition-all duration-200 hover:cursor-pointer"
                        onClick={() => router.push(item.href)}
                      >
                        <Icon className="size-4" />
                        <span className="group-data-[collapsible=icon]:hidden">
                          {item.label}
                        </span>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  );
                })}
              </SidebarMenu>
            </CollapsibleContent>
          </SidebarGroup>
        </Collapsible>
      </SidebarContent>

      {/* FOOTER */}
      <hr />
      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <div className="flex flex-row justify-between w-full align-center">
              <SidebarMenuButton
                className="rounded-2xl transition-all duration-200 hover:cursor-pointer w-auto "
                onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
              >
                {theme === "dark" ? <Moon /> : <Sun />}
                <p className="group-data-[collapsible=icon]:hidden">
                  {theme === "dark" ? "Dark" : "Light"}
                </p>
              </SidebarMenuButton>
              <p className="group-data-[collapsible=icon]:hidden text-gray-800 align-text-bottom text-[0.6rem]">
                V 0.1.0
              </p>
            </div>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}
