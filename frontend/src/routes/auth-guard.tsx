import { Navigate, useLocation } from "react-router-dom";
import { useAuthStore } from "@/stores/auth-store";

export const AuthGuard = ({ children }: { children: React.ReactNode }) => {
  const token = useAuthStore((s) => s.token);
  const location = useLocation();

  if (!token) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <>{children}</>;
};
