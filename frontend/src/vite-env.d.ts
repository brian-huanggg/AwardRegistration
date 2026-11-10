/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** External URL of the contest guide shown as the Users' "Guide" navbar item; unset hides the item. */
  readonly VITE_GUIDE_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
