import { getData, httpClient } from "@/core/http-client";
import type { TAdmin } from "@/types/admin";

export const authApi = {
  login: (username: string, password: string) =>
    getData<{ token: string }>(
      httpClient.post("auth/login", { json: { username, password } }),
    ),

  me: () => getData<TAdmin>(httpClient.get("auth/me")),
};
