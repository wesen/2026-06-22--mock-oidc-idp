import { createApi, fetchBaseQuery } from "@reduxjs/toolkit/query/react";
import type { WidgetPageResponse } from "@go-go-golems/rag-evaluation-site";

export interface AdminSession {
  subject: string;
  grant_id: string;
  grant_version: number;
  csrf_token: string;
  expires_at: string;
}

export const adminApi = createApi({
  reducerPath: "adminApi",
  baseQuery: fetchBaseQuery({
    baseUrl: "/",
    credentials: "same-origin",
  }),
  endpoints: (builder) => ({
    session: builder.query<AdminSession, void>({
      query: () => "api/admin/session",
    }),
    widgetPage: builder.query<WidgetPageResponse, { page: string; search: string }>({
      query: ({ page, search }) => `api/widget/pages/${encodeURIComponent(page)}${search}`,
    }),
  }),
});

export const { useSessionQuery, useWidgetPageQuery } = adminApi;
