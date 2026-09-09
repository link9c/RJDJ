import { create } from "zustand";
import { configApi } from "@/api";
import type { OkxConfig } from "@/api/types";

interface ConfigState {
  list: OkxConfig[];
  activeId: number | null;
  loading: boolean;
  error: string | null;
  fetch: () => Promise<void>;
  setActive: (id: number | null) => void;
  refresh: () => Promise<void>;
}

export const useConfigStore = create<ConfigState>((set, get) => ({
  list: [],
  activeId: null,
  loading: false,
  error: null,
  async fetch() {
    set({ loading: true, error: null });
    try {
      const resp = await configApi.list();
      const list = resp.data.list;
      const current = localStorage.getItem("activeConfigId");
      let activeId: number | null = current ? Number(current) : null;
      // 校验 active 是否仍存在
      if (!list.some((c) => c.id === activeId)) {
        const def = list.find((c) => c.is_default) || list[0];
        activeId = def ? def.id : null;
      }
      set({ list, activeId, loading: false });
    } catch (e) {
      set({ loading: false, error: (e as Error).message });
    }
  },
  setActive(id) {
    if (id) localStorage.setItem("activeConfigId", String(id));
    else localStorage.removeItem("activeConfigId");
    set({ activeId: id });
  },
  async refresh() {
    await get().fetch();
  },
}));
