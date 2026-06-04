export type TDecimal = string;

export type TApiResponse<T> = {
  code: number;
  message: string;
  data: T;
};

export type TPagedList<T> = {
  list: T[];
  total: number;
  page?: number;
  size?: number;
};
