import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ToastContainer } from "@/components/common/toast-container";
import { ThemeProvider } from "@/core/theme/theme-provider";
import { useToastStore } from "@/stores/toast-store";
import { AppRouter } from "@/routes";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 30_000,
    },
  },
});

export const App = () => {
  const toasts = useToastStore((s) => s.toasts);
  const dismiss = useToastStore((s) => s.dismiss);

  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <AppRouter />
        <ToastContainer toasts={toasts} onDismiss={dismiss} />
      </ThemeProvider>
    </QueryClientProvider>
  );
};
