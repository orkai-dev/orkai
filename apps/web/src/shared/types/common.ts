export interface PaginatedResponse<T> {
  items: T[];
  pagination: { total: number; page: number; per_page: number };
}
