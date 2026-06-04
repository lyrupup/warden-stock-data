import { useMutation, useQuery } from "@tanstack/react-query";
import { authApi } from "../api";
import { useAuthStore } from "@/stores/auth-store";

export const useLogin = () => {
  const login = useAuthStore((s) => s.login);

  return useMutation({
    mutationFn: ({
      username,
      password,
    }: {
      username: string;
      password: string;
    }) => authApi.login(username, password),
    onSuccess: async (data) => {
      login(data.token);
      const admin = await authApi.me();
      useAuthStore.getState().setAdmin(admin);
    },
  });
};

export const useAdminMe = (enabled: boolean) =>
  useQuery({
    queryKey: ["admin", "me"],
    queryFn: authApi.me,
    enabled,
  });
