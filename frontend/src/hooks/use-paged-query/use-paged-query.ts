import { useState } from "react";

export const usePagedQuery = (initialSize = 20) => {
  const [page, setPage] = useState(1);
  const [size] = useState(initialSize);

  return {
    page,
    size,
    setPage,
    next: () => setPage((p) => p + 1),
    prev: () => setPage((p) => Math.max(1, p - 1)),
    reset: () => setPage(1),
  };
};
