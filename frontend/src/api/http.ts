import axios from "axios";
import type { AxiosInstance } from "axios";

// 创建 axios 实例，附带 JWT
export const http: AxiosInstance = axios.create({
  baseURL: "/api",
  timeout: 120000,
});

// 请求拦截：附加 token
http.interceptors.request.use((config) => {
  const token = localStorage.getItem("token");
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// 响应拦截：统一错误提示 + 401 处理
http.interceptors.response.use(
  (resp) => resp,
  (error) => {
    const status = error.response?.status;
    const msg =
      error.response?.data?.error || error.message || "请求失败，请稍后重试";
    if (status === 401) {
      localStorage.removeItem("token");
      localStorage.removeItem("user");
      // 若不在登录页则跳转
      if (!location.pathname.startsWith("/login")) {
        location.href = "/login";
      }
    }
    return Promise.reject(new Error(msg));
  }
);

export interface ApiResp<T = unknown> {
  data: T;
}
