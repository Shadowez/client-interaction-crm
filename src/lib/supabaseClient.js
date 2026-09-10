import { createClient } from "@supabase/supabase-js";

const supabaseUrl = import.meta.env.VITE_SUPABASE_URL;
const supabasePublishableKey =
  import.meta.env.VITE_SUPABASE_PUBLISHABLE_KEY ||
  import.meta.env.VITE_SUPABASE_ANON_KEY;

export const supabaseConfigurationError =
  !supabaseUrl || !supabasePublishableKey
    ? "Supabase is not configured. Run `npm run setup` or copy `.env.example` to `.env.local` and add your project URL and publishable key."
    : "";

export const supabase = createClient(
  supabaseUrl || "https://example.invalid",
  supabasePublishableKey || "sb_publishable_not_configured"
);
