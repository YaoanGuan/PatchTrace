/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** `http` 走真实 Go 服务；缺省或其它值走内存 mock。 */
  readonly VITE_API_MODE?: string;
  /** 真实 API 基址，默认同源（由 Vite proxy 转发 `/api`）。 */
  readonly VITE_API_BASE_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
