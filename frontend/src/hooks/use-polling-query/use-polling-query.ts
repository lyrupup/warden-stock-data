import { useQuery, type UseQueryOptions } from "@tanstack/react-query";

type TUsePollingQueryOptions<T> = Omit<
  UseQueryOptions<T>,
  "refetchInterval"
> & {
  shouldPoll: (data: T | undefined) => boolean;
  intervalMs?: number;
};

export const usePollingQuery = <T>({
  shouldPoll,
  intervalMs = 2000,
  ...options
}: TUsePollingQueryOptions<T>) =>
  useQuery({
    ...options,
    refetchInterval: (query) =>
      shouldPoll(query.state.data) ? intervalMs : false,
  });
