import { useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

export default function Callback() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  useEffect(() => {
    const token = searchParams.get("token");
    if (token) {
      localStorage.setItem("token", token);
      navigate("/home", { replace: true });
    } else {
      navigate("/", { replace: true });
    }
  }, [navigate, searchParams]);

  return (
    <div className="page">
      <p>Logging in...</p>
    </div>
  );
}
