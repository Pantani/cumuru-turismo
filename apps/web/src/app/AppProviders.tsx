import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { type PropsWithChildren, useState } from "react";

import { AuthSessionProvider } from "../shared/auth/AuthSession";
import { LocaleProvider } from "../shared/i18n/LocaleProvider";

export function AppProviders({ children }: PropsWithChildren) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            retry: 1,
            staleTime: 30_000,
            refetchOnWindowFocus: false,
          },
        },
      }),
  );

  return (
    <QueryClientProvider client={queryClient}>
      <LocaleProvider>
        <AuthSessionProvider>{children}</AuthSessionProvider>
      </LocaleProvider>
    </QueryClientProvider>
  );
}
