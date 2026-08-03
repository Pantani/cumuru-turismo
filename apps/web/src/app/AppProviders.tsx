import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { type PropsWithChildren, useState } from "react";

import {
  AuthSessionProvider,
  localDemoAccessToken,
} from "../shared/auth/AuthSession";

export function AppProviders({ children }: PropsWithChildren) {
  const localDemo = import.meta.env.VITE_LOCAL_DEMO_MODE === "true";
  const accessToken = localDemoAccessToken(
    localDemo,
    import.meta.env.VITE_LOCAL_DEMO_IDENTITY,
    window.location.hostname,
  );
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
      <AuthSessionProvider accessToken={accessToken} localDemo={localDemo}>
        {children}
      </AuthSessionProvider>
    </QueryClientProvider>
  );
}
