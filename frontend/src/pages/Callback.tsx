import { useEffect } from "react";
import { useNavigate } from "react-router-dom";

export default function Callback() {
  const navigate = useNavigate();

  useEffect(() => {
    // The session cookie is set by the backend automatically.
    // Just redirect to the home page.
    navigate("/home", { replace: true });
  }, [navigate]);

  return (
    <div className="page">
      <p>Logging in...</p>
    </div>
  );
}
