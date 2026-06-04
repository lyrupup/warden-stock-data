import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { credentialsApi } from "../api";
import type { TCreateCredentialReq } from "@/types/admin";

export const credentialKeys = {
  all: ["credentials"] as const,
  list: (page: number, size: number) =>
    [...credentialKeys.all, "list", page, size] as const,
  detail: (id: number) => [...credentialKeys.all, "detail", id] as const,
};

export const useCredentials = (page: number, size: number) =>
  useQuery({
    queryKey: credentialKeys.list(page, size),
    queryFn: () => credentialsApi.list(page, size),
  });

export const useCredentialDetail = (id: number) =>
  useQuery({
    queryKey: credentialKeys.detail(id),
    queryFn: () => credentialsApi.get(id),
    enabled: id > 0,
  });

export const useCreateCredential = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: TCreateCredentialReq) => credentialsApi.create(body),
    onSuccess: () => void qc.invalidateQueries({ queryKey: credentialKeys.all }),
  });
};

export const useRotateCredential = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => credentialsApi.rotate(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey: credentialKeys.all }),
  });
};

export const useRevokeCredential = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => credentialsApi.revoke(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey: credentialKeys.all }),
  });
};

export const useUpdateCredential = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      ...body
    }: {
      id: number;
      rate_limit?: number;
      daily_quota?: number;
      status?: number;
    }) => credentialsApi.update(id, body),
    onSuccess: () => void qc.invalidateQueries({ queryKey: credentialKeys.all }),
  });
};
