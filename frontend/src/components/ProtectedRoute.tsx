import { Navigate, Outlet } from "react-router-dom";
import { useAuthContext } from "../contexts/AuthContext";

export default function ProtectedRoute() {
  const { loading, authenticated } = useAuthContext();

  if (loading) {
    return (
      <div className="page">
        <p>Loading...</p>
      </div>
    );
  }

  if (!authenticated) {
    return <Navigate to="/" replace />;
  }

  return <Outlet />;
}
