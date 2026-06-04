import ky, { type KyInstance } from "ky";
import { useAuthStore } from "@/stores/auth-store";
import type { TApiResponse } from "@/types/api";

export class AppError extends Error {
  constructor(
    public code: number,
    message: string,
  ) {
    super(message);
    this.name = "AppError";
  }
}

const baseUrl =
  import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080/admin";

const normalizedPrefix = baseUrl.endsWith("/") ? baseUrl : `${baseUrl}/`;

export const httpClient: KyInstance = ky.create({
  prefix: normalizedPrefix,
  timeout: 30_000,
  hooks: {
    beforeRequest: [
      ({ request }) => {
        const token = useAuthStore.getState().token;
        if (token) request.headers.set("Authorization", `Bearer ${token}`);
      },
    ],
    afterResponse: [
      async ({ response }) => {
        if (!response.headers.get("content-type")?.includes("application/json")) {
          return;
        }
        const body = (await response.clone().json()) as TApiResponse<unknown>;
        if (body.code === 40001) {
          useAuthStore.getState().logout();
        }
        if (body.code !== 0) {
          throw new AppError(body.code, body.message);
        }
      },
    ],
  },
});

export const getData = async <T>(
  promise: Promise<{ json: () => Promise<TApiResponse<T>> }>,
): Promise<T> => {
  const res = await promise;
  const body = await res.json();
  return body.data;
};
