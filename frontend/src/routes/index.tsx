import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { LoginPage } from "@/features/auth";
import { CredentialDetailPage, CredentialsPage } from "@/features/credentials";
import { MarketPage, StockQuotePage } from "@/features/market";
import {
  DashboardPage,
  DatasourcesPage,
  JobsPage,
} from "@/features/ops";
import { useAuthStore } from "@/stores/auth-store";
import { AppLayout } from "./app-layout";
import { AuthGuard } from "./auth-guard";

const PublicOnly = ({ children }: { children: React.ReactNode }) => {
  const token = useAuthStore((s) => s.token);
  if (token) return <Navigate to="/" replace />;
  return <>{children}</>;
};

export const AppRouter = () => (
  <BrowserRouter>
    <Routes>
      <Route
        path="/login"
        element={
          <PublicOnly>
            <LoginPage />
          </PublicOnly>
        }
      />
      <Route
        element={
          <AuthGuard>
            <AppLayout />
          </AuthGuard>
        }
      >
        <Route index element={<DashboardPage />} />
        <Route path="credentials" element={<CredentialsPage />} />
        <Route path="credentials/:id" element={<CredentialDetailPage />} />
        <Route path="market" element={<MarketPage />} />
        <Route path="market/quote" element={<StockQuotePage />} />
        <Route path="ops/datasources" element={<DatasourcesPage />} />
        <Route path="ops/jobs" element={<JobsPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  </BrowserRouter>
);
