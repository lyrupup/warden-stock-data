import { getData, httpClient } from "@/core/http-client";
import type { TPagedList } from "@/types/api";
import type {
  TCreateCredentialReq,
  TCredential,
  TCredentialDetail,
  TCredentialSecret,
} from "@/types/admin";

export const credentialsApi = {
  list: (page: number, size: number) =>
    getData<TPagedList<TCredential>>(
      httpClient.get("credentials", { searchParams: { page, size } }),
    ),

  get: (id: number) =>
    getData<TCredentialDetail>(httpClient.get(`credentials/${id}`)),

  create: (body: TCreateCredentialReq) =>
    getData<TCredentialSecret>(httpClient.post("credentials", { json: body })),

  update: (
    id: number,
    body: Partial<{
      rate_limit: number;
      daily_quota: number;
      status: number;
      expire_at: string | null;
    }>,
  ) => getData<unknown>(httpClient.put(`credentials/${id}`, { json: body })),

  revoke: (id: number) =>
    getData<unknown>(httpClient.delete(`credentials/${id}`)),

  rotate: (id: number) =>
    getData<TCredentialSecret>(
      httpClient.post(`credentials/${id}/rotate`),
    ),
};
