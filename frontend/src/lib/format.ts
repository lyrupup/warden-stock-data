import dayjs from "dayjs";

export const formatDateTime = (v: string | null | undefined): string =>
  v ? dayjs(v).format("YYYY-MM-DD HH:mm:ss") : "—";

export const formatDate = (v: string | null | undefined): string =>
  v ? dayjs(v).format("YYYY-MM-DD") : "—";

export const maskSecretId = (secretId: string): string => {
  if (secretId.length <= 8) return secretId;
  return `${secretId.slice(0, 4)}****${secretId.slice(-4)}`;
};
