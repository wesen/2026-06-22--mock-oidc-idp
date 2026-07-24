import { createApi, fetchBaseQuery } from "@reduxjs/toolkit/query/react";
import type { WidgetPageResponse } from "@go-go-golems/rag-evaluation-site";

export interface AdminSession {
  subject: string;
  grant_id: string;
  grant_version: number;
  csrf_token: string;
  expires_at: string;
}

export interface AdminListItem {
  id: string;
  primary: string;
  secondary: string;
  status: string;
}

export interface AdminPageData {
  id: string;
  title: string;
  items?: AdminListItem[];
}

export interface OneTimeSecretResult {
  resource_id: string;
  secret: string;
}

export interface ClientDetail {
  id: string;
  public: boolean;
  disabled: boolean;
  require_pkce: boolean;
  secret_configured: boolean;
  updated_at: string;
  version: number;
  redirect_uris: string[];
  post_logout_redirect_uris: string[];
  allowed_scopes: string[];
  allowed_grant_types: string[];
  allowed_audiences: string[];
  can_introspect: boolean;
}

export interface PreparedAction {
  action_handle: string;
  command: string;
  target_id?: string;
  expected_version?: number;
  require_fresh: boolean;
  require_reason: boolean;
  confirmation_text?: string;
  expires_at: string;
}

export interface DownloadGrant {
  download_handle: string;
  download_name: string;
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
    pageData: builder.query<AdminPageData, { page: string; search: string }>({
      query: ({ page, search }) => `api/admin/pages/${encodeURIComponent(page)}${search}`,
    }),
    clientDetail: builder.query<ClientDetail, string>({
      query: (clientId) => `api/admin/clients/${encodeURIComponent(clientId)}`,
    }),
    prepareAction: builder.mutation<
      PreparedAction,
      { command: string; target_id?: string; csrf: string }
    >({
      query: ({ command, target_id, csrf }) => ({
        url: "api/widget/actions/prepare",
        method: "POST",
        headers: { "X-CSRF-Token": csrf },
        body: { command, target_id: target_id ?? "" },
      }),
    }),
    executeAction: builder.mutation<
      unknown,
      { prepared: PreparedAction; input: Record<string, unknown>; csrf: string }
    >({
      query: ({ prepared, input, csrf }) => ({
        url: "api/widget/actions/execute",
        method: "POST",
        headers: {
          "X-CSRF-Token": csrf,
          "Idempotency-Key": crypto.randomUUID(),
          "X-Request-ID": crypto.randomUUID(),
        },
        body: { payload: { actionHandle: prepared.action_handle, input } },
      }),
    }),
    issueDownload: builder.mutation<
      DownloadGrant,
      { operationId: string; csrf: string }
    >({
      query: ({ operationId, csrf }) => ({
        url: `api/admin/operations/${encodeURIComponent(operationId)}/downloads`,
        method: "POST",
        headers: { "X-CSRF-Token": csrf },
      }),
    }),
  }),
});

export const {
  useSessionQuery,
  useWidgetPageQuery,
  usePageDataQuery,
  useClientDetailQuery,
  usePrepareActionMutation,
  useExecuteActionMutation,
  useIssueDownloadMutation,
} = adminApi;
